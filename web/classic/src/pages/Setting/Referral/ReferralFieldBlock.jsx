import React from 'react';
import { Typography } from '@douyinfe/semi-ui';

const { Text } = Typography;

const ReferralFieldBlock = ({ label, description, detail, note, children }) => {
  return (
    <div className='space-y-2 rounded-xl border border-gray-200 bg-gray-50/60 p-3'>
      <div className='flex items-start justify-between gap-3'>
        <div className='space-y-1'>
          <Text strong>{label}</Text>
          <div>
            <Text type='secondary'>{description}</Text>
          </div>
          {detail ? (
            <div>
              <Text type='tertiary'>{detail}</Text>
            </div>
          ) : null}
        </div>
        {note ? <Text type='tertiary'>{note}</Text> : null}
      </div>
      {children}
    </div>
  );
};

export default ReferralFieldBlock;
