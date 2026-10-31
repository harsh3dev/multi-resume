# syntax=docker/dockerfile:1
#
# Resume Editor backend: Go server + sandboxed LaTeX compile toolchain.
#
# Build:  docker build -t multi-resume-backend ./backend
# Run:    docker run --rm -p 8080:8080 --env-file .env multi-resume-backend
#
# The runtime image carries TeX Live (medium-ish scheme + resume-common
# packages), latexmk, poppler-utils (pdfinfo/pdftotext), bubblewrap, and
# prlimit (util-linux). The server shells out to bwrap per compile job;
# bubblewrap needs unprivileged user namespaces, which Debian kernels enable
# by default (kernel.unprivileged_userns_clone=1).

FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
      # TeX Live: base + recommended ≈ scheme-medium; extras cover packages
      # common in resumes: enumitem, titlesec, geometry, hyperref, xcolor,
      # tabularx, fancyhdr, marvosym, fontawesome5, charter/newtx, fullpage.
      texlive-latex-base \
      texlive-latex-recommended \
      texlive-latex-extra \
      texlive-plain-generic \
      # XeLaTeX / LuaLaTeX engines + common fonts.
      texlive-xetex \
      texlive-luatex \
      texlive-fonts-recommended \
      texlive-fonts-extra \
      latexmk \
      # pdfinfo / pdftotext for page count and probe-compile text extraction.
      poppler-utils \
      # Sandbox + resource limits.
      bubblewrap \
      util-linux \
      ca-certificates \
      curl \
    && rm -rf /var/lib/apt/lists/*

# Run unprivileged; bwrap creates its own namespaces from here.
RUN useradd --create-home --uid 10001 --shell /usr/sbin/nologin app
USER app
WORKDIR /app

COPY --from=build /out/server /app/server

ENV PORT=8080
EXPOSE 8080
CMD ["/app/server"]
