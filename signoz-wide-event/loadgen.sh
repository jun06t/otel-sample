#!/bin/sh
# ダッシュボードに連続した線を出すための負荷生成スクリプト。
# 正常系を主体に、一定割合で validation error / charge error を混ぜる。
#
#   TARGET      : リクエスト先 (既定: http://app:8000/order)
#   INTERVAL    : 1 リクエストごとの待ち秒数 (既定: 1)
#   ERROR_RATE  : 異常系の割合 % (既定: 15。内訳は validation/charge で半々)
set -eu

TARGET="${TARGET:-http://app:8000/order}"
INTERVAL="${INTERVAL:-1}"
ERROR_RATE="${ERROR_RATE:-15}"

USERS="alice bob carol dave erin frank grace heidi"
USER_COUNT=$(echo "$USERS" | wc -w)

# busybox ash には $RANDOM が無いので /dev/urandom から引く。
rand() { # rand <上限> -> 0..上限-1
  od -An -N2 -tu2 < /dev/urandom | tr -d ' \n' | awk -v n="$1" '{print $1 % n}'
}
pick_user() {
  i=$(( $(rand "$USER_COUNT") + 1 ))
  echo "$USERS" | cut -d' ' -f"$i"
}

echo "loadgen: target=$TARGET interval=${INTERVAL}s error_rate=${ERROR_RATE}%"
while :; do
  user=$(pick_user)
  roll=$(rand 100)
  half=$(( ERROR_RATE / 2 ))

  if [ "$roll" -lt "$half" ]; then
    # validation error: items <= 0
    query="user=$user&items=0&amount=$(( $(rand 9000) + 500 ))"
  elif [ "$roll" -lt "$ERROR_RATE" ]; then
    # charge error: amount > 10000
    query="user=$user&items=1&amount=$(( $(rand 90000) + 10001 ))"
  else
    query="user=$user&items=$(( $(rand 5) + 1 ))&amount=$(( $(rand 9000) + 100 ))"
  fi

  curl -s -o /dev/null "$TARGET?$query" || echo "loadgen: request failed"
  sleep "$INTERVAL"
done
