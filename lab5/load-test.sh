#!/bin/bash
FAIL=0
for i in {1..1000}; do
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 http://127.0.0.1:30080/health)
  
  if [ "$HTTP_CODE" != "200" ]; then
    echo "Ошибка! Код: $HTTP_CODE"
    FAIL=$((FAIL+1))
  fi
  
  if [ $((i % 200)) -eq 0 ]; then
    echo "Сделано $i запросов, ошибок: $FAIL"
  fi
  sleep 0.05
done
echo "ИТОГО:  запросов, ошибок: $FAIL"
