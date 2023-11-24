# Using Go image as the builder stage
FROM golang:1.21 as builder

# Set the working directory
WORKDIR /app

# Copy project files to the working directory
COPY . .

# Build the executable
RUN go build -o patch-k8s-server

# Create the final image using a lightweight Alpine image
FROM alpine:3.14

# Set the working directory
WORKDIR /app

# Copy the executable from the builder stage
COPY --from=builder /app/patch-k8s-server .

# Expose the application port
EXPOSE 8080

# Start the application
CMD ["./patch-k8s-server"]
