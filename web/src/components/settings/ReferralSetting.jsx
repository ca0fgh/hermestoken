import React from 'react';
import { Card } from '@douyinfe/semi-ui';
import SettingsReferralTemplates from '../../pages/Setting/Referral/SettingsReferralTemplates';
import SettingsReferralEngineRoutes from '../../pages/Setting/Referral/SettingsReferralEngineRoutes';

const ReferralSetting = () => {
  return (
    <>
      <Card style={{ marginTop: '10px' }}>
        <SettingsReferralTemplates />
      </Card>
      <Card style={{ marginTop: '10px' }}>
        <SettingsReferralEngineRoutes />
      </Card>
    </>
  );
};

export default ReferralSetting;
