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

import React, {
  useState,
  useCallback,
  useMemo,
  useEffect,
  useRef,
} from 'react';
import {
  Button,
  Input,
  InputNumber,
  Checkbox,
  Select,
  Typography,
  Popconfirm,
} from '@douyinfe/semi-ui';
import { IconPlus, IconDelete } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import CardTable from '../../../../components/common/ui/CardTable';
import { getSyncedDraftValue, shouldCommitDraftValue } from './groupTableDraft';

const { Text } = Typography;

const canonicalStatusOptions = [
  { label: '启用', value: 1 },
  { label: '弃用', value: 2 },
  { label: '归档', value: 3 },
];

let _idCounter = 0;
const uid = () => `gr_${++_idCounter}`;

function parseJSON(str, fallback) {
  if (!str || !str.trim()) return fallback;
  try {
    return JSON.parse(str);
  } catch {
    return fallback;
  }
}

function buildRows(groupRatioStr, userUsableGroupsStr) {
  const ratioMap = parseJSON(groupRatioStr, {});
  const usableMap = parseJSON(userUsableGroupsStr, {});

  const allNames = new Set([
    ...Object.keys(ratioMap),
    ...Object.keys(usableMap),
  ]);

  return Array.from(allNames).map((name) => ({
    _id: uid(),
    name,
    ratio: ratioMap[name] ?? 1,
    selectable: name in usableMap,
    description: usableMap[name] ?? '',
  }));
}

function buildCanonicalRows(groups = []) {
  return (groups || []).map((group) => ({
    _id: group.group_key || uid(),
    group_key: group.group_key || '',
    display_name: group.display_name || '',
    billing_ratio: Number(group.billing_ratio ?? 1),
    user_selectable: Boolean(group.user_selectable),
    description: group.description || '',
    sort_order: Number(group.sort_order ?? 0),
    status: Number(group.status ?? 1),
    _existing: Boolean(group.group_key),
  }));
}

function EditableTextCell({ value, placeholder, status, onCommit }) {
  const [draftValue, setDraftValue] = useState(value ?? '');
  const isComposingRef = useRef(false);

  useEffect(() => {
    setDraftValue((currentDraft) =>
      getSyncedDraftValue({
        committedValue: value,
        draftValue: currentDraft,
        isComposing: isComposingRef.current,
      }),
    );
  }, [value]);

  const commitValue = useCallback(
    (nextValue) => {
      if (
        shouldCommitDraftValue({
          committedValue: value,
          draftValue: nextValue,
        })
      ) {
        onCommit?.(nextValue);
      }
    },
    [onCommit, value],
  );

  const handleChange = useCallback(
    (nextValue) => {
      const normalizedValue = nextValue ?? '';
      setDraftValue(normalizedValue);
      if (!isComposingRef.current) {
        commitValue(normalizedValue);
      }
    },
    [commitValue],
  );

  const handleCompositionStart = useCallback(() => {
    isComposingRef.current = true;
  }, []);

  const handleCompositionEnd = useCallback(
    (event) => {
      isComposingRef.current = false;
      const nextValue = event?.target?.value ?? draftValue;
      setDraftValue(nextValue);
      commitValue(nextValue);
    },
    [commitValue, draftValue],
  );

  const handleBlur = useCallback(() => {
    if (!isComposingRef.current) {
      commitValue(draftValue);
    }
  }, [commitValue, draftValue]);

  return (
    <Input
      size='small'
      value={draftValue}
      status={status}
      placeholder={placeholder}
      onChange={handleChange}
      onCompositionStart={handleCompositionStart}
      onCompositionEnd={handleCompositionEnd}
      onBlur={handleBlur}
    />
  );
}

export function serializeGroupTable(rows) {
  const groupRatio = {};
  const userUsableGroups = {};

  rows.forEach((row) => {
    if (!row.name) return;
    groupRatio[row.name] = row.ratio;
    if (row.selectable) {
      userUsableGroups[row.name] = row.description;
    }
  });

  return {
    GroupRatio: JSON.stringify(groupRatio, null, 2),
    UserUsableGroups: JSON.stringify(userUsableGroups, null, 2),
  };
}

function sanitizeCanonicalRows(rows) {
  return rows
    .map((row, index) => ({
      group_key: `${row.group_key || ''}`.trim(),
      display_name: `${row.display_name || ''}`.trim(),
      billing_ratio: Number(row.billing_ratio ?? 1),
      user_selectable: Boolean(row.user_selectable),
      description: `${row.description || ''}`.trim(),
      sort_order: Number(row.sort_order ?? index),
      status: Number(row.status ?? 1),
      _existing: Boolean(row._existing),
      _id: row._id,
    }))
    .filter((row) => row.group_key);
}

export default function GroupTable({
  groupRatio,
  userUsableGroups,
  groups,
  onChange,
  mode = 'legacy',
}) {
  const { t } = useTranslation();

  const [rows, setRows] = useState(() =>
    mode === 'canonical'
      ? buildCanonicalRows(groups)
      : buildRows(groupRatio, userUsableGroups),
  );

  useEffect(() => {
    if (mode === 'canonical') {
      setRows(buildCanonicalRows(groups));
      return;
    }
    setRows(buildRows(groupRatio, userUsableGroups));
  }, [mode, groups, groupRatio, userUsableGroups]);

  const emitChange = useCallback(
    (newRows) => {
      setRows(newRows);
      if (mode === 'canonical') {
        onChange?.(sanitizeCanonicalRows(newRows));
        return;
      }
      onChange?.(serializeGroupTable(newRows));
    },
    [mode, onChange],
  );

  const updateRow = useCallback(
    (id, field, value) => {
      const next = rows.map((r) =>
        r._id === id ? { ...r, [field]: value } : r,
      );
      emitChange(next);
    },
    [rows, emitChange],
  );

  const addRow = useCallback(() => {
    const existingNames = new Set(
      rows.map((r) => (mode === 'canonical' ? r.group_key : r.name)),
    );
    let counter = 1;
    let newName = `group_${counter}`;
    while (existingNames.has(newName)) {
      counter++;
      newName = `group_${counter}`;
    }
    if (mode === 'canonical') {
      emitChange([
        ...rows,
        {
          _id: uid(),
          group_key: newName,
          display_name: newName,
          billing_ratio: 1,
          user_selectable: true,
          description: '',
          sort_order: rows.length,
          status: 1,
          _existing: false,
        },
      ]);
      return;
    }
    emitChange([
      ...rows,
      {
        _id: uid(),
        name: newName,
        ratio: 1,
        selectable: true,
        description: '',
      },
    ]);
  }, [rows, emitChange, mode]);

  const removeRow = useCallback(
    (id) => {
      emitChange(rows.filter((r) => r._id !== id));
    },
    [rows, emitChange],
  );

  const groupNames = useMemo(
    () => rows.map((r) => (mode === 'canonical' ? r.group_key : r.name)),
    [rows, mode],
  );

  const duplicateNames = useMemo(() => {
    const counts = {};
    groupNames.forEach((n) => {
      counts[n] = (counts[n] || 0) + 1;
    });
    return new Set(Object.keys(counts).filter((k) => counts[k] > 1));
  }, [groupNames]);

  const columns = useMemo(() => {
    if (mode === 'canonical') {
      return [
        {
          title: t('分组键'),
          dataIndex: 'group_key',
          key: 'group_key',
          width: 180,
          render: (_, record) =>
            record._existing ? (
              <Text>{record.group_key}</Text>
            ) : (
              <EditableTextCell
                value={record.group_key}
                status={
                  duplicateNames.has(record.group_key) ? 'warning' : undefined
                }
                onCommit={(v) => updateRow(record._id, 'group_key', v)}
              />
            ),
        },
        {
          title: t('显示名'),
          dataIndex: 'display_name',
          key: 'display_name',
          render: (_, record) => (
            <EditableTextCell
              value={record.display_name}
              placeholder={t('显示名称')}
              onCommit={(v) => updateRow(record._id, 'display_name', v)}
            />
          ),
        },
        {
          title: t('倍率'),
          dataIndex: 'billing_ratio',
          key: 'billing_ratio',
          width: 120,
          render: (_, record) => (
            <InputNumber
              size='small'
              min={0}
              step={0.1}
              value={record.billing_ratio}
              style={{ width: '100%' }}
              onChange={(v) => updateRow(record._id, 'billing_ratio', v ?? 0)}
            />
          ),
        },
        {
          title: t('排序'),
          dataIndex: 'sort_order',
          key: 'sort_order',
          width: 90,
          render: (_, record) => (
            <InputNumber
              size='small'
              precision={0}
              value={record.sort_order}
              style={{ width: '100%' }}
              onChange={(v) => updateRow(record._id, 'sort_order', v ?? 0)}
            />
          ),
        },
        {
          title: t('状态'),
          dataIndex: 'status',
          key: 'status',
          width: 110,
          render: (_, record) => (
            <Select
              size='small'
              value={record.status}
              optionList={canonicalStatusOptions}
              onChange={(value) => updateRow(record._id, 'status', value)}
            />
          ),
        },
        {
          title: t('用户可选'),
          dataIndex: 'user_selectable',
          key: 'user_selectable',
          width: 90,
          align: 'center',
          render: (_, record) => (
            <Checkbox
              checked={record.user_selectable}
              onChange={(e) =>
                updateRow(record._id, 'user_selectable', e.target.checked)
              }
            />
          ),
        },
        {
          title: t('描述'),
          dataIndex: 'description',
          key: 'description',
          render: (_, record) => (
            <EditableTextCell
              value={record.description}
              placeholder={t('描述')}
              onCommit={(v) => updateRow(record._id, 'description', v)}
            />
          ),
        },
        {
          title: '',
          key: 'actions',
          width: 50,
          render: (_, record) => (
            <Popconfirm
              title={t('确认归档该分组？')}
              onConfirm={() => removeRow(record._id)}
              position='left'
            >
              <Button
                icon={<IconDelete />}
                type='danger'
                theme='borderless'
                size='small'
              />
            </Popconfirm>
          ),
        },
      ];
    }

    return [
      {
        title: t('分组名称'),
        dataIndex: 'name',
        key: 'name',
        width: 180,
        render: (_, record) => (
          <EditableTextCell
            value={record.name}
            status={duplicateNames.has(record.name) ? 'warning' : undefined}
            onCommit={(v) => updateRow(record._id, 'name', v)}
          />
        ),
      },
      {
        title: t('倍率'),
        dataIndex: 'ratio',
        key: 'ratio',
        width: 120,
        render: (_, record) => (
          <InputNumber
            size='small'
            min={0}
            step={0.1}
            value={record.ratio}
            style={{ width: '100%' }}
            onChange={(v) => updateRow(record._id, 'ratio', v ?? 0)}
          />
        ),
      },
      {
        title: t('用户可选'),
        dataIndex: 'selectable',
        key: 'selectable',
        width: 90,
        align: 'center',
        render: (_, record) => (
          <Checkbox
            checked={record.selectable}
            onChange={(e) =>
              updateRow(record._id, 'selectable', e.target.checked)
            }
          />
        ),
      },
      {
        title: t('描述'),
        dataIndex: 'description',
        key: 'description',
        render: (_, record) =>
          record.selectable ? (
            <EditableTextCell
              value={record.description}
              placeholder={t('分组描述')}
              onCommit={(v) => updateRow(record._id, 'description', v)}
            />
          ) : (
            <Text type='tertiary' size='small'>
              -
            </Text>
          ),
      },
      {
        title: '',
        key: 'actions',
        width: 50,
        render: (_, record) => (
          <Popconfirm
            title={t('确认删除该分组？')}
            onConfirm={() => removeRow(record._id)}
            position='left'
          >
            <Button
              icon={<IconDelete />}
              type='danger'
              theme='borderless'
              size='small'
            />
          </Popconfirm>
        ),
      },
    ];
  }, [t, duplicateNames, updateRow, removeRow, mode]);

  return (
    <div>
      <CardTable
        columns={columns}
        dataSource={rows}
        rowKey='_id'
        hidePagination
        size='small'
        empty={<Text type='tertiary'>{t('暂无分组，点击下方按钮添加')}</Text>}
      />
      <div className='mt-3 flex justify-center'>
        <Button icon={<IconPlus />} theme='outline' onClick={addRow}>
          {t('添加分组')}
        </Button>
      </div>
      {duplicateNames.size > 0 && (
        <Text type='warning' size='small' className='mt-2 block'>
          {t('存在重复的分组名称：')}
          {Array.from(duplicateNames).join(', ')}
        </Text>
      )}
    </div>
  );
}
