# TODO

需包含两种访问OSS中数据的方式：
1. 通过OSS存储卷挂载访问
2. 通过OSS SDK访问

## 通过OSS存储卷挂载访问

引用阿里云官网OSS存储卷配置文档

需包含：
1. 如何在Ray Job中指定volume / volumeMount挂载OSS存储卷
2. 访问volumeMount路径的Ray Data示例代码 （实现一个简单的读取 -> 处理 -> 写入 Pipeline）

## 通过OSS SDK访问

需包含：
1. 安装了OSS SDK的Dockerfile
2. 访问OSS中数据的Ray Data示例代码 （实现一个简单的读取 -> 处理 -> 写入 Pipeline）