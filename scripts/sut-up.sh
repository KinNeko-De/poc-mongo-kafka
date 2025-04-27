#!/bin/bash

param="$1"
docker compose -p cluster -f sut/kafka.yaml -f sut/mongodb.yaml up $param