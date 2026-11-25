#!/bin/bash
# Runs at the start of the e2e SSH target: sshd accepts the remote forwards
# (ssh -R) the gateway opens for e2e step W4. The first value of a keyword
# wins in sshd_config, so the line goes on top.
sed -i '/^AllowTcpForwarding /d; 1i AllowTcpForwarding yes' /config/sshd/sshd_config
