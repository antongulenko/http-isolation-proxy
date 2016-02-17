package services

import (
	"errors"
	"fmt"
	"time"
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
)

type RedisLock struct {
	Client     Redis
	LockName   string
	LockValue  string // Should be unique per client
	Expiration time.Duration
	Retry      uint
}

func RegisterLockScripts(client Redis) error {
	var err error
	if lua_sha_unlock, err = client.Cmd("SCRIPT", "LOAD", lua_script_unlock).Str(); err != nil {
		return err
	}
	return nil
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

func (lock *RedisLock) Transaction(transaction func(redis Redis) error) error {
	if err := lock.Lock(); err != nil {
		return err
	}
	err := lock.Client.Transaction(func(redis Redis) error {
		if err := transaction(redis); err != nil {
			return err
		}
		return lock.UnlockIn(redis)
	})
	if err != nil {
		// Transaction failed, try to unlock
		if unlockErr := lock.Unlock(); unlockErr != nil {
			L.Warnf("Lock-transaction failed and failed to unlock %v (%v): %v", lock.LockName, lock.LockValue, unlockErr)
		}
	}
	return err
}

func (lock *RedisLock) LockedDo(do func()) error {
	if err := lock.Lock(); err != nil {
		return err
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			L.Warnf("Failed to unlock lock %v (%v): %v", lock.LockName, lock.LockValue, err)
		}
	}()
	do()
	return nil
}

func (lock *RedisLock) Unlock() error {
	return lock.UnlockIn(lock.Client)
}

func (lock *RedisLock) UnlockIn(redis Redis) error {
	return redis.Cmd("evalsha", lua_sha_unlock, 1, lock.LockName, lock.LockValue).Err()
}
