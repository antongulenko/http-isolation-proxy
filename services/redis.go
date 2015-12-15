package services

import (
	"fmt"

	"github.com/mediocregopher/radix.v2/pool"
	impl "github.com/mediocregopher/radix.v2/redis"
)

const (
	poolSize = 20
)

type Redis interface {
	Cmd(cmd string, args ...interface{}) RedisResponse
	Transaction(transaction func() error) error
	StoreStruct(key string, obj interface{}) error
	LoadStruct(key string, obj interface{}) error
}

type RedisResponse interface {
	HasResult() bool
	Err() error
	Map() (map[string]string, error)
	Str() (string, error)
	Int() (int, error)
	List() ([]string, error)
	Bool() (bool, error)
}

func ConnectRedis(endpoint string) (Redis, error) {
	client, err := pool.New("tcp", endpoint, poolSize)
	if err != nil {
		return nil, err
	}
	return &redis{client}, nil
}

type redis struct {
	client *pool.Pool
}

type redisResponse struct {
	*impl.Resp
}

func (r redis) Cmd(cmd string, args ...interface{}) RedisResponse {
	return redisResponse{
		Resp: r.client.Cmd(cmd, args...),
	}
}

func (resp redisResponse) Bool() (bool, error) {
	i, err := resp.Int()
	return i == 1, err
}

func (resp redisResponse) Err() error {
	return resp.Resp.Err
}

func (resp redisResponse) HasResult() bool {
	return !resp.IsType(impl.Nil)
}

func (r redis) Transaction(transaction func() error) error {
	conn, err := r.client.Get()
	if err != nil {
		return err
	}
	defer r.client.Put(conn)

	if err := conn.Cmd("multi").Err; err != nil {
		return fmt.Errorf("Failed to start redis transaction: %v", err)
	}

	if err := transaction(); err != nil {
		if abort_err := conn.Cmd("discard").Err; abort_err != nil {
			return fmt.Errorf("%v. Error aborting transaction: %v", err, abort_err)
		} else {
			return err
		}
	}

	if err := conn.Cmd("exec").Err; err != nil {
		return fmt.Errorf("Failed to commit redis transaction: %v", err)
	}

	return nil
}