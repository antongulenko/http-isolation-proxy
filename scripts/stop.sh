#!/bin/bash
source "$HOME/.bash_localrc"
while IFS='' read -r line || [[ -n "$line" ]]; do
    test -z "$line" && continue
    arr=($line)
    program=${arr[0]}
    if [ "$program" = "sudo" ]; then
        program=${arr[1]}
        line="sudo `which $program` ${arr[@]:2}"
    fi
    sudo pkill $program
done < "$HOME/services"
