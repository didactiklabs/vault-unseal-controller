{
  pkgs ? import <nixpkgs> { },
}:
let
  packages = with pkgs; [
    kustomize
    kubebuilder
  ];

  allPackages = packages;
  # Generate the list of packages with their versions
  packageList = builtins.concatStringsSep "\n" (
    builtins.map (
      pkg:
      let
        name = pkg.pname or (builtins.parseDrvName pkg.name).name;
        version = pkg.version or "unknown";
      in
      "echo -e '  \\033[32m✓\\033[0m ${name}\\t\\033[90m${version}\\033[0m'"
    ) allPackages
  );

  # List all available binaries in bin/
  binariesList = ''
    echo ""
    echo -e "\033[1;33m🔧 Available programs:\033[0m"
    for pkg in ${toString allPackages}; do
      if [ -d "$pkg/bin" ]; then
        for bin in "$pkg/bin"/*; do
          if [ -x "$bin" ]; then
            echo "  $(basename "$bin")"
          fi
        done
      fi
    done | sort -u
  '';
in
pkgs.mkShell {
  buildInputs = allPackages;

  shellHook = ''
    echo ""
    echo -e "\033[1;33m📦 Available tools (${toString (builtins.length allPackages)} packages):\033[0m"
    ${packageList}
    ${binariesList}
    echo ""
  '';
}
