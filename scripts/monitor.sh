#!/bin/bash
bank_endpoint="bank:9001"
isolator_endpoint="bank:7777"
redis_host="redis"
watch -n 1 \
    "curl -s $bank_endpoint/stats;
    echo;
    curl -s $isolator_endpoint/runtime;
    echo;
    echo -n 'Open orders: ';
    redis-cli -h $redis_host scard open_orders;
    echo;
    curl -s $isolator_endpoint/stats"

