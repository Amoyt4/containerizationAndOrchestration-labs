#!/bin/bash

FAULT=$((RANDOM % 3))

case $FAULT in
  0)
    sed -i 's/port: 5432/port: 5433/' chart/values.yaml
    ;;
  1)
    sed -i 's/port: 53/port: 54/' chart/values.yaml
    ;;
  2)
    sed -i 's/port: 8080/port: 8081/' chart/values.yaml
    ;;
esac

helm upgrade shop ./chart -n shop > /dev/null
echo "Что то поломалось..."
