#!/bin/bash

CELL=${CELL:-'test'}

export CELL=$CELL
export NS=$NS

echo "Stop ==========================================tidb"
./tidb-down.sh
echo "Stop ==========================================tikv"
./tikv-down.sh
echo "Stop ============================================pd"
./pd-down.sh

