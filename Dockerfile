FROM golang as BUILDER

WORKDIR /tmp/project
COPY ./ .
RUN go build cmd/main.go

FROM golang

RUN mkdir -p /app/

WORKDIR /app

# copy build files
COPY --from=BUILDER /tmp/project/main /app/

# Create non-root user and use it as the default user
RUN adduser --shell /sbin/nologin app && chown -R app:app /app
USER app

CMD ["/app/main"]