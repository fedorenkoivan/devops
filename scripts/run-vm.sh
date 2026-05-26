#!/usr/bin/env bash
qemu-system-aarch64 \
  -M virt,highmem=on \
  -cpu max \
  -accel hvf \
  -smp 2 \
  -m 2048 \
  -bios /opt/homebrew/share/qemu/edk2-aarch64-code.fd \
  -drive file=ubuntu-server.qcow2,if=virtio,format=qcow2 \
  -netdev user,id=net0,hostfwd=tcp::8080-:80,hostfwd=tcp::2222-:22 \
  -device virtio-net-device,netdev=net0 \
  -serial stdio \
  -display none