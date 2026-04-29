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
*/

import React, { useEffect, useState } from 'react';
import { Modal, TextArea, Typography } from '@douyinfe/semi-ui';

const { Text } = Typography;

const WithdrawalReviewModal = ({
  visible,
  onCancel,
  onSubmit,
  record,
  mode,
  t,
}) => {
  const [note, setNote] = useState('');

  useEffect(() => {
    if (visible) {
      setNote('');
    }
  }, [visible]);

  return (
    <Modal
      visible={visible}
      title={mode === 'approve' ? t('审核通过提现申请') : t('驳回提现申请')}
      onCancel={onCancel}
      onOk={() => onSubmit(note)}
      centered
    >
      <div className='space-y-3'>
        <Text type='tertiary'>
          {record?.trade_no} ·{' '}
          {record?.username || `#${record?.user_id || '--'}`}
        </Text>
        <TextArea
          value={note}
          onChange={setNote}
          autosize={{ minRows: 4 }}
          placeholder={
            mode === 'approve' ? t('可选填写审核备注') : t('请填写驳回原因')
          }
        />
      </div>
    </Modal>
  );
};

export default WithdrawalReviewModal;
