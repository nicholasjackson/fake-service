#!/bin/bash

# Wait until Consul can be contacted
until curl -s ${CONSUL_HTTP_ADDR}/v1/status/leader | grep 8300; do
  echo "Waiting for Consul to start"
  sleep 1
done

# If we do not need to register a service just run the command
if [ ! -z "$SERVICE_CONFIG" ]; then
  # register the service with consul
  echo "Registering service with consul $SERVICE_CONFIG"
  consul services register ${SERVICE_CONFIG}
  
  exit_status=$?
  if [ $exit_status -ne 0 ]; then
    echo "### Error writing service config: $file ###"
    cat $file
    echo ""
    exit 1
  fi
fi

# register any central config from individual files
if [ ! -z "$CENTRAL_CONFIG" ]; then
  IFS=';' read -r -a configs <<< ${CENTRAL_CONFIG}

  for file in "${configs[@]}"; do
    echo "Writing central config $file"
    consul config write $file
     
    exit_status=$?
    if [ $exit_status -ne 0 ]; then
      echo "### Error writing central config: $file ###"
      cat $file
      echo ""
      exit 1
    fi
  done
fi

# register any central config from a folder
if [ ! -z "$CENTRAL_CONFIG_DIR" ]; then
  for file in `ls -v $CENTRAL_CONFIG_DIR/*`; do 
    echo "Writing central config $file"
    consul config write $file
    echo ""

    exit_status=$?
    if [ $exit_status -ne 0 ]; then
      echo "### Error writing central config: $file ###"
      cat $file
      echo ""
      exit 1
    fi
  done
fi
