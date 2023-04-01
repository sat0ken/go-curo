#!/bin/bash

# rootユーザーが必要
if [ $UID -ne 0 ]; then
  echo "Root privileges are required"
  exit 1;
fi

# 全てのnetnsを削除
ip -all netns delete

# bridgeを作成
ip link add br100 type bridge

# 4つのnetnsを作成
ip netns add host1
ip netns add host2
ip netns add router1
ip netns add router2
ip netns add host3

# リンクの作成
ip link add name host1-br100 type veth peer name br100-host1 # host1とbr0のリンク
ip link add name host2-br100 type veth peer name br100-host2 # host2とbr0のリンク
ip link add name router1-br100 type veth peer name br100-router1 # router1とbr0のリンク
ip link add name router1-router2 type veth peer name router2-router1 # router1とrouter2のリンク
ip link add name router2-host3 type veth peer name host3-router2 # router2とhost3のリンク

# ブリッジする
ip link set dev br100-host1 master br100
ip link set dev br100-host2 master br100
ip link set dev br100-router1 master br100
ip link set br100-host1 up
ip link set br100-host2 up
ip link set br100-router1 up
ip link set br100 up

# リンクの割り当て
ip link set host1-br100 netns host1
ip link set host2-br100 netns host2
ip link set router1-br100 netns router1
ip link set router1-router2 netns router1
ip link set router2-router1 netns router2
ip link set router2-host3 netns router2
ip link set host3-router2 netns host3

# host1のリンクの設定
ip netns exec host1 ip addr add 192.168.1.3/24 dev host1-br100
ip netns exec host1 ip link set host1-br100 up
ip netns exec host1 ethtool -K host1-br100 rx off tx off
ip netns exec host1 ip route add default via 192.168.1.1

# host2のリンクの設定
ip netns exec host2 ip addr add 192.168.1.2/24 dev host2-br100
ip netns exec host2 ip link set host2-br100 up
ip netns exec host2 ethtool -K host2-br100 rx off tx off
ip netns exec host2 ip route add default via 192.168.1.1


# router1のリンクの設定
ip netns exec router1 ip link set router1-br100 up
ip netns exec router1 ethtool -K router1-br100 rx off tx off
ip netns exec router1 ip link set router1-router2 up
ip netns exec router1 ethtool -K router1-router2 rx off tx off
# router1のipv4フォワードを無効にする
ip netns exec router1 sysctl -w net.ipv4.ip_forward=0

# goでrouter1のip設定するのが面倒くさいので追加
ip netns exec router1 ip addr add 192.168.1.1/24 dev router1-br100
ip netns exec router1 ip link set router1-br100 up
ip netns exec router1 ethtool -K router1-br100 rx off tx off
ip netns exec router1 ip addr add 192.168.0.1/24 dev router1-router2
ip netns exec router1 ip link set router1-router2 up
ip netns exec router1 ethtool -K router1-router2 rx off tx off
# ルータ起動する時はルートは不要
# ip netns exec router1 ip route add 192.168.2.0/24 via 192.168.0.1

# router2のリンクの設定
ip netns exec router2 ip addr add 192.168.0.2/24 dev router2-router1
ip netns exec router2 ip link set router2-router1 up
ip netns exec router2 ethtool -K router2-router1 rx off tx off
ip netns exec router2 ip route add 192.168.1.0/24 via 192.168.0.1
ip netns exec router2 ip addr add 192.168.2.1/24 dev router2-host3
ip netns exec router2 ip link set router2-host3 up
ip netns exec router2 ethtool -K router2-host3 rx off tx off
#ip netns exec router2 sysctl -w net.ipv4.ip_forward=1

# host3のリンクの設定
ip netns exec host3 ip addr add 192.168.2.2/24 dev host3-router2
ip netns exec host3 ip link set host3-router2 up
ip netns exec host3 ethtool -K host3-router2 rx off tx off
ip netns exec host3 ip route add default via 192.168.2.1
