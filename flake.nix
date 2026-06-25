{
  description = "Daytona development environments";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };

        # macOS Apple SDK — provides Security, SystemConfiguration, CoreFoundation, etc.
        # Required for CGO (Go) and crypto libraries.
        # In recent nixpkgs the legacy per-framework imports (darwin.apple_sdk.frameworks.*)
        # have been removed in favor of the unified apple-sdk package.
        darwinDeps = pkgs.lib.optionals pkgs.stdenv.isDarwin [
          pkgs.apple-sdk
          (pkgs.darwinMinVersionHook "11.0")
        ];

        # Yarn 4.x wrapper — delegates to corepack bundled with Node.js
        # The project pins yarn via package.json "packageManager": "yarn@4.13.0"
        yarnWrapper = pkgs.writeShellScriptBin "yarn" ''
          exec ${pkgs.nodejs_22}/bin/corepack yarn "$@"
        '';

        # ──────────────────────────────────────────────
        # Shared packages (included in every shell)
        # ──────────────────────────────────────────────
        commonPkgs = with pkgs; [
          git
          curl
          jq
          gnumake
          pkg-config
        ];

        # ──────────────────────────────────────────────
        # Go toolchain
        # Covers: apps/{daemon,proxy,runner,snapshot-manager,ssh-gateway,otel-collector/exporter}
        #         libs/{api-client-go,common-go,computer-use}
        # ──────────────────────────────────────────────
        goPkgs = with pkgs; [
          go # 1.25.x — matches go.work constraint
          golangci-lint
          protobuf # provides protoc
          buf
          protoc-gen-go
          protoc-gen-go-grpc
          libgit2
        ] ++ darwinDeps ++ bpfPkgs;

        goShellHook = ''
          unset GOROOT
          export GOPATH="''${GOPATH:-$HOME/go}"
          export GOBIN="$GOPATH/bin"
          export PATH="$GOBIN:$PATH"

          # Install Go tools not packaged in nixpkgs
          _nix_install_go_tool() {
            local name="$1" pkg="$2"
            if ! command -v "$name" &>/dev/null; then
              echo "nix-shell: installing $name ..."
              go install "$pkg" 2>/dev/null || echo "nix-shell: warning — failed to install $name"
            fi
          }
          _nix_install_go_tool swag      "github.com/swaggo/swag/cmd/swag@v1.16.4"
          _nix_install_go_tool gow       "github.com/mitranim/gow@v0.0.0-20260225145757-ff0f6779ab4c"
          _nix_install_go_tool gomarkdoc "github.com/princjef/gomarkdoc/cmd/gomarkdoc@v1.1.0"
          unset -f _nix_install_go_tool
        '';

        # ──────────────────────────────────────────────
        # eBPF toolchain (Linux only)
        # Covers: libs/netleash — `make generate` runs bpf2go, which compiles the
        # BPF C sources with clang and strips them with llvm-strip. libbpf and the
        # kernel UAPI headers supply <bpf/...> and <linux/...>/<asm/...>.
        # Pinned to LLVM 18 to match the committed generated objects.
        # The header packages go in buildInputs (not packages) so the clang
        # cc-wrapper injects their include dirs via NIX_CFLAGS_COMPILE — this lets
        # `make generate` find the headers without any Makefile changes.
        # ──────────────────────────────────────────────
        bpfPkgs = pkgs.lib.optionals pkgs.stdenv.isLinux [
          pkgs.llvmPackages_18.clang # bpf2go: clang -cc
          pkgs.llvmPackages_18.llvm # bpf2go: llvm-strip
        ];

        bpfHeaderInputs = pkgs.lib.optionals pkgs.stdenv.isLinux [
          pkgs.libbpf # <bpf/bpf_helpers.h>, <bpf/bpf_endian.h>
          pkgs.linuxHeaders # <linux/bpf.h>, <asm/types.h>, ...
        ];

        # ──────────────────────────────────────────────
        # Node.js / TypeScript toolchain
        # Covers: apps/{api,dashboard}
        # ──────────────────────────────────────────────
        nodePkgs = [
          pkgs.nodejs_22
          yarnWrapper
        ];

        nodeShellHook = ''
          export NX_DAEMON=true
          export NODE_ENV=development
          export COREPACK_ENABLE_DOWNLOAD_PROMPT=0
          export COREPACK_HOME="''${COREPACK_HOME:-$HOME/.cache/corepack}"
          mkdir -p "$COREPACK_HOME"
        '';

        # ──────────────────────────────────────────────
        # Java runtime for OpenAPI generator
        # ──────────────────────────────────────────────
        javaPkgs = [
          pkgs.jdk17
        ];

        javaShellHook = ''
          export JAVA_HOME="${pkgs.jdk17.home}"
        '';

      in
      {
        devShells = {

          # Full monorepo
          default = pkgs.mkShell {
            name = "daytona";
            packages = commonPkgs ++ goPkgs ++ nodePkgs ++ javaPkgs;
            buildInputs = bpfHeaderInputs;
            # bpf2go invokes clang with `-target bpf`; the cc-wrapper's hardening
            # flags (e.g. -fzero-call-used-regs) are unsupported for that target.
            hardeningDisable = [ "all" ];
            shellHook = ''
              ${goShellHook}
              ${nodeShellHook}
              ${javaShellHook}
            '';
          };

          # Go services and libraries only
          go = pkgs.mkShell {
            name = "daytona-go";
            packages = commonPkgs ++ goPkgs;
            buildInputs = bpfHeaderInputs;
            # bpf2go invokes clang with `-target bpf`; the cc-wrapper's hardening
            # flags (e.g. -fzero-call-used-regs) are unsupported for that target.
            hardeningDisable = [ "all" ];
            shellHook = goShellHook;
          };

          # TypeScript / Node.js apps and libraries only
          node = pkgs.mkShell {
            name = "daytona-node";
            packages = commonPkgs ++ nodePkgs;
            shellHook = nodeShellHook;
          };

        };
      }
    );
}
