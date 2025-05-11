yeet.setenv("KO_DOCKER_REPO", "ghcr.io/jasonlovesdoggo/jsn");
yeet.setenv("SOURCE_DATE_EPOCH", $`git log -1 --format='%ct'`.trim());
yeet.setenv("VERSION", git.tag());

programs = ["serve", "httpdebug", "pkg.jsn.cam"].join(",");

$`ko build --platform=all --base-import-paths --tags=latest,${git.tag()} ./cmd/{${programs}}`;

yeet.setenv("CGO_ENABLED", "0");

const pkgs = [];

["amd64", "arm64"].forEach((goarch) => {
  [deb, rpm].forEach((method) => {
    pkgs.push(
      method.build({
        name: "httpdebug",
        description: "HTTP protocol debugger",
        homepage: "https://jsn.cam",
        license: "aGPLv3",
        goarch,

        documentation: {
          LICENSE: "LICENSE",
        },

        configFiles: {
          "cmd/httpdebug/httpdebug.env": "/etc/.jsn/httpdebug.env",
        },

        build: ({ bin, systemd }) => {
          $`go build -o ${bin}/httpdebug -ldflags '-s -w -extldflags "-static" -X "github.com/jasonlovesdoggo/jsn.Version=${git.tag()}"' ./cmd/httpdebug`;
          file.install(
            "./cmd/httpdebug/httpdebug.service",
            `${systemd}/httpdebug.service`,
          );
        },
      }),
    );

    pkgs.push(
      method.build({
        name: "serve",
        description: "Like python3 -m http.server but a single binary",
        homepage: "https://jsn.cam",
        license: "aGPLv3",
        goarch,

        documentation: {
          LICENSE: "LICENSE",
        },

        build: ({ bin }) => {
          $`go build -o ${bin}/serve -ldflags '-s -w -extldflags "-static" -X "github.com/jasonlovesdoggo/jsn.Version=${git.tag()}"' ./cmd/serve`;
        },
      }),
    );
  });
});
