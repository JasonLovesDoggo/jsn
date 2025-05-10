["amd64", "arm64"].forEach((goarch) => {
  [deb, rpm, tarball].forEach((method) =>
    method.build({
      name: "serve",
      description: "Like python3 -m http.server but a single binary",
      homepage: "https://github.com/jasonlovesdoggo/jsn",
      license: "aGPLv3",
      goarch,

      documentation: {
        "../../LICENSE": "LICENSE",
      },

      build: ({ bin }) => {
        $`go build -o ${bin}/serve -ldflags '-s -w -extldflags "-static" -X "github.com/jasonlovesdoggo/jsn.Version=${git.tag()}"'`;
      },
    }),
  );
});
