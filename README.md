# tinylog

## プロジェクト概要

**tinylog** は、Go/TinyGoで実装されたMQTTベースの分散ログシステムです。  
MQTTクライアントとして機能し、以下を並列に実行することでの自律的にデータを蓄積します。
- Publisherとしてデータをクラスタネットワークに送信
- Subscriber/Raftノードとしてデータログを安定的に保持
最終的にIoTデバイス(マイコン)への移植を想定し、TinyGoでビルド可能にすることを目指しています。

- Wi-Fi接続に対応したマイコン(Raspberry Pi Pico W等)での動作を想定し、TinyGoでビルド可能(作業中)


## 使い方

### 開発環境構築

1. **Devcontainerを使う場合**  
   VSCodeで `.devcontainer/devcontainer.json` を参照してコンテナを起動
   - 作業用の `tinylog-app`コンテナと MQTTブローカ `tinylog-mosquitto`コンテナが同時に起動します
   - ポート 1883 から Moquittoに接続

2. **ローカルビルド**

```bash
# protocでAPIコード生成
make protoc

# amd64用バイナリをビルド
make build

# TinyGo対応の組み込み用バイナリをビルド
make tinybuild
```