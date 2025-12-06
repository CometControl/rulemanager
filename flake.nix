{
  description = "Go development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go Tools
            go             # The Go programming language
            gopls          # Go language server (IDE support)
            delve          # Go debugger
            golangci-lint  # Linter
            gotools        # Additional tools (goimports, etc.)
            gofumpt        # Stricter gofmt
            
            # Build Tools
            gnumake        # Make build system
            git            # Version control
            docker         # Docker CLI
            docker-compose # Container orchestration
            
            # Database
            mongosh        # MongoDB Shell (Client only)
          ];

          shellHook = ''
            echo "Go development environment loaded!"
            echo "Run 'docker-compose up -d' to start the database."
            go version
          '';
        };
      }
    );
}
