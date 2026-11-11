# syntax=docker/dockerfile:1
FROM golang:1.27.4-trixie@sha256:b54b2639de8c1bfe538ec5ffc45f29b1932235ac5f6d63c059853646677112d9 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/rolecue ./cmd/api

FROM debian:trixie-slim@sha256:f6b08b09a05725f9431502d3b5714f5c4576e82c1a84fa1debe0c24a7e0c3ec4 AS blender
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl xz-utils unzip \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /downloads
# Blender's download host challenges automated clients; use its RWTH download mirror.
RUN curl -fsSL --retry 3 https://ftp.halifax.rwth-aachen.de/blender/release/Blender5.2/blender-5.2.2-linux-x64.tar.xz -o blender.tar.xz \
    && echo '84098912789dc450e95697c4184fb8a90acbe5111c2ba4aede3fecb57806a168  blender.tar.xz' | sha256sum -c - \
    && mkdir /opt/blender && tar -xJf blender.tar.xz -C /opt/blender --strip-components=1 \
    && curl -fsSL --retry 3 https://github.com/saturday06/VRM-Addon-for-Blender/releases/download/v4.10.2/VRM_Addon_for_Blender-Extension-4_10_2.zip -o vrm.zip \
    && echo '228e622d158fbbf070b1b49547708f360a223873f00fa81a42170794492e6db9  vrm.zip' | sha256sum -c - \
    && mkdir -p /opt/blender-extensions/user_default/vrm \
    && unzip -q vrm.zip -d /opt/blender-extensions/user_default/vrm
# Patch Python libraries shipped in Blender's archive, including setuptools vendors.
# Hash-locked (api/blender-requirements.txt): pip refuses any artifact whose sha256 differs.
COPY blender-requirements.txt /tmp/blender-requirements.txt
RUN /opt/blender/5.2/python/bin/python3.13 -m pip install --no-cache-dir --no-deps --only-binary=:all: \
    --require-hashes -r /tmp/blender-requirements.txt \
    && rm /tmp/blender-requirements.txt

FROM cgr.dev/chainguard/wolfi-base:latest@sha256:360896efb0753a5701c48f860e17130c205eae5a264797aa3e7ea126ee2923a7 AS runtime
RUN apk add --no-cache --upgrade \
    ca-certificates poppler-utils libglvnd libx11 libxxf86vm libxfixes libxi \
    libxrender libxkbcommon libsm libice libgomp dbus-libs wayland libgcc libstdc++ expat \
    && adduser -D -u 10001 rolecue
COPY --from=blender /opt/blender /opt/blender
COPY --from=blender --chown=rolecue:rolecue /opt/blender-extensions /home/rolecue/.config/blender/5.2/extensions
ENV HOME=/home/rolecue PATH=/opt/blender:$PATH
WORKDIR /app
COPY --from=build /out/rolecue ./rolecue
COPY --chown=rolecue:rolecue configs ./configs
USER rolecue
# Assert the runtime contract as the same user that handles conversion requests.
RUN blender --background --factory-startup --disable-autoexec --python-exit-code 1 \
    --python-expr 'import addon_utils,bpy,tomllib; from pathlib import Path; m=addon_utils.enable("bl_ext.user_default.vrm",default_set=True); assert m is not None; assert bpy.app.version[:2]==(5,2); assert tomllib.loads(Path(m.__file__).with_name("blender_manifest.toml").read_text())["version"]=="4.10.2"; bpy.ops.export_scene.vrm.get_rna_type()'
EXPOSE 8080
CMD ["./rolecue"]
