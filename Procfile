0: sleep infinity # allow to stop and start the other nodes without goreman exiting

1: go run ./cmd/vespiary/ --data-dir /tmp/vespiary1 --debug -n 3 --serf-port 2790 --raft-port $(( $PORT + 1000 )) -j 127.0.0.1:2790 -j 127.0.0.1:2792 --rpc-tls-certificate-file ./run_config/cert.pem --rpc-tls-private-key-file ./run_config/privkey.pem --rpc-tls-certificate-authority-file ./run_config/cert.pem --mtls --metrics-port 8089
2: go run ./cmd/vespiary/ --data-dir /tmp/vespiary2 --debug -n 3 --serf-port 2791 --raft-port $(( $PORT + 1000 )) -j 127.0.0.1:2790 -j 127.0.0.1:2792 --rpc-tls-certificate-file ./run_config/cert.pem --rpc-tls-private-key-file ./run_config/privkey.pem --rpc-tls-certificate-authority-file ./run_config/cert.pem --mtls
3: go run ./cmd/vespiary/ --data-dir /tmp/vespiary3 --debug -n 3 --serf-port 2792 --raft-port $(( $PORT + 1000 )) -j 127.0.0.1:2790 -j 127.0.0.1:2791 --rpc-tls-certificate-file ./run_config/cert.pem --rpc-tls-private-key-file ./run_config/privkey.pem --rpc-tls-certificate-authority-file ./run_config/cert.pem --mtls
