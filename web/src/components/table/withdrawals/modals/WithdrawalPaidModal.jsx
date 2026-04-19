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

import React, { useEffect, useState } from 'react';
import { Input, Modal, TextArea, Typography } from '@douyinfe/semi-ui';

const { Text } = Typography;

const WithdrawalPaidModal = ({ visible, onCancel, onSubmit, record, t }) => {
  const [payReceiptNo, setPayReceiptNo] = useState('');
  const [payReceiptUrl, setPayReceiptUrl] = useState('');
  const [paidNote, setPaidNote] = useState('');

  useEffect(() => {
    if (visible) {
      setPayReceiptNo('');
      setPayReceiptUrl('');
      setPaidNote('');
    }
  }, [visible]);

  return (
    <Modal
      visible={visible}
      title={t('确认已打款')}
      onCancel={onCancel}
      onOk={() =>
        onSubmit({
          payReceiptNo,
          payReceiptUrl,
          paidNote,
        })
      }
      centered
    >
      <div className='space-y-3'>
        <Text type='tertiary'>
          {record?.trade_no} ·{' '}
          {record?.username || `#${record?.user_id || '--'}`}
        </Text>
        <Input
          value={payReceiptNo}
          onChange={setPayReceiptNo}
          placeholder={t('回执号 / 支付宝流水号（可选）')}
        />
        <Input
          value={payReceiptUrl}
          onChange={setPayReceiptUrl}
          placeholder={t('回执图片链接（可选）')}
        />
        <TextArea
          value={paidNote}
          onChange={setPaidNote}
          autosize={{ minRows: 4 }}
          placeholder={t('打款备注（可选）')}
        />
      </div>
    </Modal>
  );
};

export default WithdrawalPaidModal;
