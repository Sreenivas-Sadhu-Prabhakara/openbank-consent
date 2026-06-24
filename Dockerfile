# Self-contained build: the shared library lives in ./pkg (replace directive),
# so the whole repo is the build context — no external module path needed.
FROM golang:1.25-alpine AS build
WORKDIR /app
COPY . .
RUN go build -trimpath -o /out/service .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 svc
USER svc
COPY --from=build /out/service /service
ENTRYPOINT ["/service"]
