/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
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
