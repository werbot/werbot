FROM golang:alpine AS build
RUN apk add --no-cache make git upx bash ca-certificates && update-ca-certificates 2>/dev/null || true
ENV PATH=$PATH:$GOROOT/bin:$GOPATH/bin
ENV USER=_NAME_
ENV UID=10001
RUN adduser --disabled-password --gecos "" --home "/nonexistent" --shell "/sbin/nologin" --no-create-home --uid "${UID}" "${USER}"
WORKDIR /app
ADD . .
RUN make prod_build _NAME_

FROM scratch
LABEL com.werbot.commit _GIT_COMMIT_
LABEL com.werbot.version _VERSION_
LABEL org.opencontainers.image.source https://github.com/werbot/werbot
LABEL org.opencontainers.image.description _DESCRIPTION_
LABEL org.opencontainers.image.licenses Business Source License 1.1
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/group /etc/group
COPY --from=build /app/bin/_NAME_ /
USER _NAME_:_NAME_
ENTRYPOINT ["/_NAME_"]
