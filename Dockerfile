FROM golang:1.23.10-alpine AS builder
RUN mkdir /build
COPY go.mod go.sum *.go /build/
COPY docs /build/docs
WORKDIR /build
RUN go mod tidy
RUN go build -o bin/wombo-combo-go-be

FROM alpine
COPY --from=builder /build/bin/wombo-combo-go-be /app/
COPY icons /app/icons
COPY Combinations.csv /app/Combinations.csv
COPY Words.csv /app/Words.csv
WORKDIR /app
ENV WORDS=/app/Words.csv
ENV COMBINATIONS=/app/Combinations.csv
ENV ICONS=/app/icons
ENV DB="POSTGRES"
CMD ["./wombo-combo-go-be"]
