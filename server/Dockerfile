FROM golang:1.18-alpine

# Install protoc and the protoc-gen-go and protoc-gen-go-grpc plugins
RUN apk add --no-cache protobuf git

# Set the working directory
WORKDIR /grpc_stream-medium/server

# Copy the source code into the container
COPY . .

RUN go get github.com/go-redis/redis
RUN go mod download google.golang.org/protobuf
RUN go mod download google.golang.org/grpc
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go
RUN go get google.golang.org/grpc/cmd/protoc-gen-go-grpc
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
# Generate Go code from .proto files
RUN protoc --go_out=. --go-grpc_out=require_unimplemented_servers=false:. *.proto

RUN go mod tidy

# CMD ["/bin/sh", "-c", "sleep infinity"]

# Build the Go application
RUN go build -o main .

# # Expose the port on which the application will run
# EXPOSE 8080

# # Command to run the executable
CMD ["./main"]