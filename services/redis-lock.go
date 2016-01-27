package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/pborman/uuid"
)

var LockFailed = errors.New("Failed to acquire lock")

var (
	lua_sha_unlock  = ""
	lua_sha_execute = ""
)

const (
	lua_script_unlock = `
	local res = redis.call("get",KEYS[1])
	if res == ARGV[1] then
	    return redis.call("del",KEYS[1])
	elseif res == false then
		return {err = 'Lock does not exist: ' .. KEYS[1]}
	else
	    return {err = 'Lock is owned by other key'}
	end
	`

	lua_script_execute = `
	local res = redis.call("get",KEYS[1])
	if res == KEYS[2] then
	    return redis.call(unpack(ARGV))
	elseif res == false then
		return {err = 'Lock does not exist: ' .. KEYS[1]}
	else
	    return {err = 'Lock is owned by other key'}
	end
	`
)

type RedisLock struct {
	Client     Redis
	LockName   string
	LockValue  string // Should be unique per client (use LoadLockValue())
	Expiration time.Duration
	Retry      uint
}

func RegisterLockScripts(client Redis) error {
	var err error
	if lua_sha_unlock, err = client.Cmd("SCRIPT", "LOAD", lua_script_unlock).Str(); err != nil {
		return err
	}
	if lua_sha_execute, err = client.Cmd("SCRIPT", "LOAD", lua_script_execute).Str(); err != nil {
		return err
	}
	return nil
}

func (lock *RedisLock) LoadLockValue(key string) error {
	cmd := lock.Client.Cmd("get", key)
	if cmd.HasResult() {
		// Key does not exist, generate new uuid
		lock.LockValue = uuid.New()
		return lock.Client.Cmd("set", key, lock.LockValue, 0).Err()
	} else if res, err := cmd.Str(); err != nil {
		return err
	} else {
		lock.LockValue = res
		return nil
	}
}

func (lock *RedisLock) TryLock() error {
	resp := lock.Client.Cmd("set", lock.LockName, lock.LockValue, "ex", lock.Expiration.Seconds(), "nx")
	str, err := resp.Str()
	if resp.HasResult() && err == nil && str == "OK" {
		return nil
	}
	return LockFailed
}

func (lock *RedisLock) Lock() error {
	var i uint = 0
	for ; i == 0 || i < lock.Retry; i++ {
		err := lock.TryLock()
		if err == LockFailed {
			continue
		} else {
			return err
		}
	}
	if lock.Retry <= 1 {
		return LockFailed
	} else {
		return fmt.Errorf("%v, giving up after %v attempts", LockFailed, i)
	}
}

func (lock *RedisLock) Unlock() error {
	return lock.Client.Cmd("evalsha", lua_sha_unlock, 1, lock.LockName, lock.LockValue).Err()
}

func (lock *RedisLock) Execute(command ...string) RedisResponse {
	cmd := lock.Client.Cmd("evalsha", lua_sha_execute, 2, lock.LockName, lock.LockValue, command)
	return cmd
}
