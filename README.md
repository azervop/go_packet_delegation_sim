# Description

This repository simulates the behavior of a queue using packet delegation. Packet delegation is a mechanism described in the paper:
- Zervopoulos, A., & Oikonomou, K. (2023, June). Load balancing of virtual network functions through packet delegation. In 2023 International Balkan Conference on Communications and Networking (BalkanCom) (pp. 1-6). IEEE.

## (Under Construction)

## Instructions

- Run with:
    - `go run go_delegation_queue.go params/params.json`
- Multiple simulations can be executed in parallel for all parameter files in the `params` folder using the included Python file:
    - `python3 run_multiple.py`
    - __WARNING__: This will generate multiple large event logging CSVs (~25GB).
