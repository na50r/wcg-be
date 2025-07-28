FROM golang:1.23.10-alpine AS builder
RUN apk add --no-cache gcc g++ git openssh-client
RUN mkdir /build
COPY go.mod go.sum storage.go types.go api.go main.go sqlite.go account.go game.go lobby.go sse.go utility.go constants.go /build/
COPY docs /build/docs
WORKDIR /build
RUN go mod tidy

# Required to get sqlite3 to work
RUN CGO_ENABLED=1 go build -o bin/wombo-combo-go-be

FROM alpine
RUN adduser -S -D -H -h /app appuser
COPY --from=builder /build/bin/wombo-combo-go-be /app/
COPY icons /app/icons
COPY Combinations.csv /app/Combinations.csv
COPY Words.csv /app/Words.csv
WORKDIR /app
RUN chown -R appuser /app && chmod 755 /app
USER appuser
ENV WORDS=/app/Words.csv
ENV COMBINATIONS=/app/Combinations.csv
ENV ICONS=/app/icons
CMD ["./wombo-combo-go-be"]