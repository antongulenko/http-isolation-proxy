package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/antongulenko/golib"
	"github.com/pborman/uuid"
)

var LockFailed = errors.New("Failed to acquire lock")

var (
	lua_sha_unlock      = ""
	lua_sha_extend      = ""
	endpoint_lock_value string
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

	lua_script_extend = `
	local res = redis.call("get",KEYS[1])
	if res == ARGV[1] then
	    return redis.call("set",KEYS[1],ARGV[1],"ex",ARGV[2])
	elseif res == false then
	    return {err = 'Lock does not exist: ' .. KEYS[1]}
	else
	    return {err = 'Lock is owned by other key'}
	end
	`
)

func init() {
	addr, err := golib.FirstIpAddress()
	if err != nil {
		endpoint_lock_value = uuid.New()
		L.Warnf("Failed to determine IP address: %v", err)
		L.Warnf("Using random value for redis locks: %v", endpoint_lock_value)
	}
	endpoint_lock_value = addr.String()
}

type RedisLock struct {
	Client     Redis
	LockName   string
	LockValue  string // Should be unique per client
	Expiration time.Duration
	Retry      uint
}

// Return a string that is unique per client/endpoint. addr should contain service port.
func EndpointLockValue(addr string) string {
	return endpoint_lock_value + ":" + addr
}

func RegisterLockScripts(client Redis) error {
	var err error
	if lua_sha_unlock, err = client.Cmd("SCRIPT", "LOAD", lua_script_unlock).Str(); err != nil {
		return err
	}
	if lua_sha_extend, err = client.Cmd("SCRIPT", "LOAD", lua_script_extend).Str(); err != nil {
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

// At the end of the transaction, unlock the lock and make sure it is still owned by us
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
			L.Logf("Lock-transaction failed and failed to unlock %v (%v): %v", lock.LockName, lock.LockValue, unlockErr)
		}
	}
	return err
}

// Assume lock is already locked!
// Instead of unlocking in the end, make sure that we still own the lock and extend the timeout
func (lock *RedisLock) ExtendTransaction(transaction func(redis Redis) error) error {
	err := lock.Client.Transaction(func(redis Redis) error {
		if err := transaction(redis); err != nil {
			return err
		}
		return lock.ExtendIn(redis)
	})
	if err != nil {
		// Transaction failed, try to unlock
		if unlockErr := lock.Unlock(); unlockErr != nil {
			L.Logf("Extend-Lock-transaction failed and failed to unlock %v (%v): %v", lock.LockName, lock.LockValue, unlockErr)
		}
	}
	return err
}

func (lock *RedisLock) Unlock() error {
	return lock.UnlockIn(lock.Client)
}

func (lock *RedisLock) UnlockIn(redis Redis) error {
	return redis.Cmd("evalsha", lua_sha_unlock, 1, lock.LockName, lock.LockValue).Err()
}

func (lock *RedisLock) Extend() error {
	return lock.ExtendIn(lock.Client)
}

func (lock *RedisLock) ExtendIn(redis Redis) error {
	return redis.Cmd("evalsha", lua_sha_extend, 1, lock.LockName, lock.LockValue, lock.Expiration.Seconds()).Err()
}
