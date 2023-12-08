FROM golang:1.20.5-bullseye AS builder

ENV GOPROXY=https://goproxy.io

WORKDIR /app

COPY ../go.mod .
COPY ../go.sum .
COPY ./buildimage ./buildimage
COPY ../logging ./logging
COPY ../config ./config
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o s3downloader ./buildimage/cmd

FROM quay.io/buildah/stable:v1.32.2

WORKDIR /root/

COPY --from=builder /app/s3downloader .

COPY ./ray271-llama2-7b-finetune.tar .

COPY ../scripts/buildimage.sh .
RUN chmod +x buildimage.sh

ENTRYPOINT ["./buildimage.sh"]