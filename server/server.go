package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"
	"os"
	// "github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"io"
	"go.uber.org/fx"
	pb "grpc_stream-medium/server/grpc_stream-medium/server/myservice"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	// "google.golang.org/grpc/keepalive"
)

// Environment variables
type Env struct {
	RedisSentinelAddrs []string
	RedisMasterName    string
	RedisPassword      string
	ServerPort         string
	SentinelPassword  string
	Password string
}

// Redis structure
type Redis struct {
	RedisClient *redis.Client
}

// NewRedis creates a new Redis instance
func NewRedis(env Env) (*Redis, error) {
	client := redis.NewFailoverClient(&redis.FailoverOptions{
		SentinelAddrs:  env.RedisSentinelAddrs,
		MasterName:     env.RedisMasterName,
		DialTimeout:    5 * time.Second,
		SentinelPassword:       env.RedisPassword,
		Password:         env.RedisPassword,
		MaxRetries:        3,
        MinRetryBackoff:   8 * time.Millisecond,
        MaxRetryBackoff:   512 * time.Millisecond,
	})

	if client == nil {
		return nil, errors.New("failed to connect to Redis Sentinel")
	}

	return &Redis{
		RedisClient: client,
	}, nil
}

// MessageConsumer is a generic consumer for different message types.
type MessageConsumer struct {
	redisClient *Redis
}

// NewMessageConsumer creates a new instance of MessageConsumer.
func NewMessageConsumer(redis *Redis) *MessageConsumer {
	return &MessageConsumer{
		redisClient: redis,
	}
}

// gRPC server implementation
type grpcMessageServer struct {
	messageConsumer *MessageConsumer
}

func (s *grpcMessageServer) SendMessage(stream pb.MessageStream_SendMessageServer) error {
    // Implement your logic here
    for {
        // Receive message from client stream
        req, err := stream.Recv()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }

        queueNames := req.GetQueueNames()
        for _, queueName := range queueNames {
            // Subscribe to the Redis channel
            sub := s.messageConsumer.redisClient.RedisClient.Subscribe(stream.Context(), queueName)
            defer sub.Close()
            ch := sub.Channel()

			// _, err := sub.Receive(stream.Context())
			// if err != nil {
			// 	log.Printf("Error subscribing to Redis channel: %v", err)
			// 	return err
			// }

            // Continuously listen for messages from the channel
            for {
                select {
                case <-stream.Context().Done():
                    log.Printf("[%s] Client disconnected...", queueName)
                    return nil
                case msg := <-ch:
                    // Construct a message response
                    response := &pb.MessageResponse{
                        Message: msg.Payload,
                    }
                    // Send the response to the client
                    if err := stream.Send(response); err != nil {
                        log.Printf("Failed to send message to client: %v", err)
                        return err
                    }
					fmt.Printf("[%s] Received message: %+s\n", queueName, msg)
                }
            }
        }
    }
}
// Run gRPC server
func runGRPCServer(env Env, consumer *MessageConsumer) {
	grpcServer := grpc.NewServer(
		// grpc.KeepaliveParams(keepalive.ServerParameters{
		// 	Time:    20 * time.Second, // Ping the client if it is idle for 10 seconds to ensure the connection is still active
		// 	Timeout: 5 * time.Second,  // Wait 5 seconds for the ping ack before considering the connection dead
		// }),
	)
	pb.RegisterMessageStreamServer(grpcServer, &grpcMessageServer{messageConsumer: consumer})
	// Enable gRPC server reflection
	reflection.Register(grpcServer)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", env.ServerPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf("gRPC server listening on port %s", env.ServerPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func main() {
	env := Env{
		RedisSentinelAddrs: []string{os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT")},
		RedisMasterName:    "mymaster",
		RedisPassword:      os.Getenv("REDIS_PWD"),
		ServerPort:         "8080",
	}

	app := fx.New(
		fx.Provide(func() Env { return env }),
		fx.Provide(NewRedis),
		fx.Provide(NewMessageConsumer),
		fx.Invoke(func(lifecycle fx.Lifecycle, env Env, consumer *MessageConsumer) {
			// Start gRPC server
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go runGRPCServer(env, consumer)
					return nil
				},
			})
		}),
	)

	app.Run()
}
