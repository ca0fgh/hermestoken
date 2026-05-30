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

import React from 'react';
import { Button, Dropdown, Modal, Space } from '@douyinfe/semi-ui';
import { IconMore } from '@douyinfe/semi-icons';
import { showInfo } from '../../../helpers';

const stopActionEvent = (event) => {
  event?.stopPropagation?.();
};

const ChannelOperateActions = ({
  t,
  record,
  upstreamUpdateMeta,
  manageChannel,
  manageTag,
  testChannel,
  setCurrentTestChannel,
  setShowModelTestModal,
  setEditingChannel,
  setShowEdit,
  setShowEditTag,
  setEditingTag,
  copySelectedChannel,
  refresh,
  activePage,
  channels,
  checkOllamaVersion,
  setShowMultiKeyManageModal,
  setCurrentMultiKeyChannel,
  openUpstreamUpdateModal,
  detectChannelUpstreamUpdates,
}) => {
  if (record.children !== undefined) {
    return (
      <Space
        className='channel-operate-actions'
        spacing={8}
        onClick={stopActionEvent}
      >
        <Button
          type='tertiary'
          size='small'
          onClick={(event) => {
            stopActionEvent(event);
            manageTag(record.key, 'enable');
          }}
        >
          {t('启用全部')}
        </Button>
        <Button
          type='tertiary'
          size='small'
          onClick={(event) => {
            stopActionEvent(event);
            manageTag(record.key, 'disable');
          }}
        >
          {t('禁用全部')}
        </Button>
        <Button
          type='tertiary'
          size='small'
          onClick={(event) => {
            stopActionEvent(event);
            setShowEditTag(true);
            setEditingTag(record.key);
          }}
        >
          {t('编辑')}
        </Button>
      </Space>
    );
  }

  const handleEdit = (event) => {
    stopActionEvent(event);
    setEditingChannel(record);
    setShowEdit(true);
  };

  const handleDelete = () => {
    Modal.confirm({
      title: t('确定是否要删除此渠道？'),
      content: t('此修改将不可逆'),
      onOk: () => {
        (async () => {
          await manageChannel(record.id, 'delete', record);
          await refresh();
          setTimeout(() => {
            if (channels.length === 0 && activePage > 1) {
              refresh(activePage - 1);
            }
          }, 100);
        })();
      },
    });
  };

  const handleCopy = () => {
    Modal.confirm({
      title: t('确定是否要复制此渠道？'),
      content: t('复制渠道的所有信息'),
      onOk: () => copySelectedChannel(record),
    });
  };

  const handleModelTest = () => {
    setCurrentTestChannel(record);
    setShowModelTestModal(true);
  };

  const handleMultiKeyManage = () => {
    setCurrentMultiKeyChannel(record);
    setShowMultiKeyManageModal(true);
  };

  const handleApplyUpstreamUpdates = () => {
    if (!upstreamUpdateMeta?.enabled) {
      showInfo(t('该渠道未开启上游模型更新检测'));
      return;
    }
    if (
      upstreamUpdateMeta.pendingAddModels.length === 0 &&
      upstreamUpdateMeta.pendingRemoveModels.length === 0
    ) {
      showInfo(t('该渠道暂无可处理的上游模型更新'));
      return;
    }
    openUpstreamUpdateModal(
      record,
      upstreamUpdateMeta.pendingAddModels,
      upstreamUpdateMeta.pendingRemoveModels,
      upstreamUpdateMeta.pendingAddModels.length > 0 ? 'add' : 'remove',
    );
  };

  const upstreamMenuItems = upstreamUpdateMeta?.supported
    ? [
        {
          node: 'item',
          name: t('仅检测上游模型更新'),
          type: 'tertiary',
          onClick: () => detectChannelUpstreamUpdates(record),
        },
        {
          node: 'item',
          name: t('处理上游模型更新'),
          type: 'tertiary',
          onClick: handleApplyUpstreamUpdates,
        },
      ]
    : [];

  const moreMenuItems = [
    ...(record.channel_info?.is_multi_key
      ? [
          {
            node: 'item',
            name: t('多密钥管理'),
            type: 'tertiary',
            onClick: handleMultiKeyManage,
          },
        ]
      : []),
    ...(record.type === 4
      ? [
          {
            node: 'item',
            name: t('测活'),
            type: 'tertiary',
            onClick: () => checkOllamaVersion(record),
          },
        ]
      : []),
    {
      node: 'item',
      name: t('按模型测试'),
      type: 'tertiary',
      onClick: handleModelTest,
    },
    ...upstreamMenuItems,
    {
      node: 'item',
      name: t('复制'),
      type: 'tertiary',
      onClick: handleCopy,
    },
    {
      node: 'item',
      name: t('删除'),
      type: 'danger',
      onClick: handleDelete,
    },
  ];

  return (
    <Space
      className='channel-operate-actions'
      spacing={8}
      onClick={stopActionEvent}
    >
      <Button
        data-testid='channel-edit-action'
        type='tertiary'
        size='small'
        onClick={handleEdit}
      >
        {t('编辑')}
      </Button>

      <Button
        size='small'
        type='tertiary'
        onClick={(event) => {
          stopActionEvent(event);
          testChannel(record, '');
        }}
      >
        {t('测试')}
      </Button>

      {record.status === 1 ? (
        <Button
          type='danger'
          size='small'
          onClick={(event) => {
            stopActionEvent(event);
            manageChannel(record.id, 'disable', record);
          }}
        >
          {t('禁用')}
        </Button>
      ) : (
        <Button
          size='small'
          onClick={(event) => {
            stopActionEvent(event);
            manageChannel(record.id, 'enable', record);
          }}
        >
          {t('启用')}
        </Button>
      )}

      <Dropdown trigger='click' position='bottomRight' menu={moreMenuItems}>
        <Button
          aria-label={t('更多渠道操作')}
          icon={<IconMore />}
          type='tertiary'
          size='small'
        />
      </Dropdown>
    </Space>
  );
};

export default ChannelOperateActions;
