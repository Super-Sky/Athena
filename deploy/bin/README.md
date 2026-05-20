# bin

## 目录职责

- 描述 Athena 二进制部署时的目标目录约定。

## 推荐目录结构

```text
/opt/athena/
├── athena
├── athena.env
├── config/
└── deploy/
```

## 说明

- `athena`
  - 由 `scripts/build_release_bundle.sh` 生成的 Linux 可执行文件。
- `athena.env`
  - 运行时环境变量文件，供 systemd 读取。
- `config/`
  - 随发布包一起交付的基础配置目录。
- `deploy/`
  - systemd 模板和环境变量模板。
