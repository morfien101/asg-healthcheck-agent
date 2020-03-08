#!/bin/bash

if [ "$1" == "0" ]; then
    echo "Running the test script. Will exit $1"
    exit 0
else
    echo "Running the test script. Will exit 1"
    exit 1
fi