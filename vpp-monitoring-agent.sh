#!/bin/sh -

UUID=`hostid`
echo "Starting VPP monitoring agent with UUID for VPP:"
echo $UUID

STATUS=100

while [ $STATUS -eq 100 ]
do
  $(dirname $0)/vpp-monitoring-agent -debug -wiring-file=$(dirname $0)/vpp-monitoring-agent-configuration.yaml -vpp-uuid=$UUID
  STATUS=$?
  echo "Vpp-monitoring-agent exited with status: $STATUS"
  if [ $STATUS -eq 100 ]
  then
    echo "Restarting..."
  fi
done