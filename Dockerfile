FROM golang:1.18.1-bullseye AS build-env

RUN apt-get update && apt-get install -y gcc cmake pkg-config

ADD [".", "/app/"]

RUN ["/app/install_git2go.sh"]
RUN ["/app/build_goblet.sh"]
RUN ["/app/build_hooks.sh"]

FROM ubuntu:20.04
EXPOSE 8080
# ARG PEM_TEXT
# ENV GH_APP_ID=852830
# ENV GH_APP_INSTALLATION_ID=48249800
# ENV GH_APP_PRIVATE_KEY=$PEM_TEXT
RUN apt-get update && apt-get install -y git
COPY --from=build-env ["/tmp/goblet-server", "/tmp/packobjectshook", "/git2go/static-build/build/CMakeCache.txt", "/app/example_config.json", "/app/"]
WORKDIR /app
RUN ["./goblet-server", "-config", "example_config.json", "-check"]
CMD ["./goblet-server", "-config", "example_config.json"]
