["amd64", "arm64"].forEach((goarch) => {
  [deb, rpm, tarball].forEach((method) =>
    method.build({
      name: "serve",
      description: "Like python3 -m http.server but a single binary",
      homepage: "https://pkg.jsn.cam/jsn",
      license: "aGPLv3",
      goarch,

      documentation: {
        "../../LICENSE": "LICENSE",
      },

      build: ({ bin }) => {
        $`go build -o ${bin}/serve -ldflags '-s -w -extldflags "-static" -X "pkg.jsn.cam/jsn.Version=${git.tag()}"'`;
      },
    }),
  );
});
