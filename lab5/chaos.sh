#!/bin/bash
while true; do
  POD=$(kubectl get pods -n shop -l 'app.kubernetes.io/component in (api, worker)' -o jsonpath='{.items[?(@.metadata.deletionTimestamp==null)].metadata.name}' | tr ' ' '\n' | shuf -n 1)
  if [ -n "$POD" ]; then
    echo "[$(date +%T)] Убиваем под: $POD"
    kubectl delete pod -n shop "$POD" --grace-period=0 --force 2>/dev/null
  fi
  sleep 2
done
