import { Button, Result } from 'antd';
import React from 'react';

const NoQuotaPage = () => (
  <Result
    status="403"
    title=""
    subTitle="请联系管理员，分配资源配额后再重新登录"
    extra={
      <Button type="primary" onClick={() => window.location.href='/cluster'}>
        返回
      </Button>
    }
  ></Result>
);

export default NoQuotaPage;
