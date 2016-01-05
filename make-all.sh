#!/bin/bash

if [ -z "$go_command" ]; then
    go_command="install"
    if [ "$1" = "clean" ]; then
        go_command="clean -i"
    fi
fi
echo "Running: $go_command"

base="github.com/antongulenko/http-isolation-proxy"
prefix="$base/services"
go $go_command "$prefix/service_user"
go $go_command "$prefix/service_bank/bankApi"
go $go_command "$prefix/service_bank"
go $go_command "$prefix/service_catalog/catalogApi"
go $go_command "$prefix/service_catalog"
go $go_command "$prefix/service_payment/paymentApi"
go $go_command "$prefix/service_payment"
go $go_command "$prefix/service_shop/shopApi"
go $go_command "$prefix/service_shop"

go $go_command "$prefix/service_verify"
go $go_command "$prefix/service_user"

go $go_command "$base/proxy"
go $go_command "$base/isolator"
