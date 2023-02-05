# go-curo

2022年インターフェイス11月号のソフトウェアルータをgoに移植

## 元ネタ

https://github.com/kametan0730/interface_2022_11  
https://github.com/kametan0730/curo

## 実行方法

```shell
sudo ./netns-scripts/chapter4-2-netns.sh
go build .
# ルータを起動
sudp ip netns exec route1 ./main --chapter2

# 別のシェルで
sudo ip netns exec host1 ping 192.168.1.1
```