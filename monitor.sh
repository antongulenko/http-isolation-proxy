#!/bin/bash
bank_endpoint="localhost:9001"
isolator_endpoint="localhost:9006"
watch -n 1 \
    "curl -s $bank_endpoint/stats;
    echo;
    curl -s $isolator_endpoint/runtime;
    echo;
    echo -n 'Open orders: ';
    redis-cli scard open_orders;
    echo;
    curl -s $isolator_endpoint/stats"

