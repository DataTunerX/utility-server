# Using Go image as the builder stage
FROM golang:1.21 as builder

# Set the working directory
WORKDIR /app

# Set go proxy
ENV GOPROXY=https://goproxy.io

# Copy project files to the working directory
COPY . .

# Build the executable
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o patch-k8s-server

# Create the final image using a lightweight Alpine image
FROM frolvlad/alpine-bash:latest

# Set the working directory
WORKDIR /app

# Copy the executable from the builder stage
COPY --from=builder /app/patch-k8s-server .

# Make patch-k8s-server executable
RUN chmod +x patch-k8s-server

# Expose the application port
EXPOSE 8080

# Start the application
CMD ["./patch-k8s-server"]
