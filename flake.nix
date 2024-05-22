{
  description = "ARP-MQTT";

  # Nixpkgs / NixOS version to use.
  inputs.nixpkgs.url = "nixpkgs/nixos-23.11";

  outputs = {
    self,
    nixpkgs,
  }: let
    # to work with older version of flakes
    lastModifiedDate = self.lastModifiedDate or self.lastModified or "19700101";

    # Generate a user-friendly version number.
    version = builtins.substring 0 8 lastModifiedDate;

    # System types to support.
    supportedSystems = ["x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin"];

    # Helper function to generate an attrset '{ x86_64-linux = f "x86_64-linux"; ... }'.
    forAllSystems = nixpkgs.lib.genAttrs supportedSystems;

    # Nixpkgs instantiated for supported system types.
    nixpkgsFor = forAllSystems (system: import nixpkgs {inherit system;});
  in {
    # Provide some binary packages for selected system types.
    packages = forAllSystems (system: let
      pkgs = nixpkgsFor.${system};
    in {
      arp-mqtt = pkgs.buildGoModule {
        pname = "arp-mqtt";
        inherit version;

        src = ./.;

        vendorHash = "sha256-wr6PYUPzmbPQTX9fob9iX87CybJRWVao0RlVzgyiU88=";
      };
    });

    # Add dependencies that are only needed for development
    devShells = forAllSystems (system: let
      pkgs = nixpkgsFor.${system};
    in {
      default = pkgs.mkShell {
        buildInputs = with pkgs; [
          go
          gopls
          gotools
          go-tools
          go-outline
          godef
          delve
          mqttui
        ];
      };
    });

    # The default package for 'nix build'. This makes sense if the
    # flake provides only one package or there is a clear "main"
    # package.
    defaultPackage = forAllSystems (system: self.packages.${system}.arp-mqtt);
  };
}
