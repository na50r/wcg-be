FROM golang:1.23.10-alpine AS builder
RUN mkdir /build
COPY go.mod go.sum *.go /build/

# Copy packages
COPY sse/ /build/sse
COPY token/ /build/token
COPY utility/ /build/utility
COPY constants/ /build/constants
COPY dto/ /build/dto
COPY game/ /build/game
COPY account/ /build/account
COPY storage/ /build/storage

COPY docs /build/docs
WORKDIR /build
RUN go mod tidy
RUN go build -o bin/wombo-combo-go-be

FROM alpine
COPY --from=builder /build/bin/wombo-combo-go-be /app/
WORKDIR /app

# Prodive env variables but no files, seed locally
ENV WORDS=/app/Words.csv
ENV COMBINATIONS=/app/Combinations.csv
ENV ICONS=/app/icons
ENV ACHIEVEMENTS=/app/Achivements.csv
ENV ACHIEVEMENT_ICONS=/app/achievement_icons
ENV DB="POSTGRES"
CMD ["./wombo-combo-go-be"]
