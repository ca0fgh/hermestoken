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

import React, { useMemo } from 'react';

const VIEWBOX_WIDTH = 96;
const VIEWBOX_HEIGHT = 40;
const STROKE_WIDTH = 2;

function getMinMax(values) {
  const numericValues = values.filter((value) => Number.isFinite(value));
  if (numericValues.length === 0) {
    return { min: 0, max: 0 };
  }

  return {
    min: Math.min(...numericValues),
    max: Math.max(...numericValues),
  };
}

function buildSparklinePoints(data) {
  if (!Array.isArray(data) || data.length === 0) {
    return '';
  }

  const { min, max } = getMinMax(data);
  const safeRange = max - min || 1;

  return data
    .map((value, index) => {
      const x =
        data.length === 1
          ? VIEWBOX_WIDTH / 2
          : (index / (data.length - 1)) * VIEWBOX_WIDTH;
      const normalizedValue = (value - min) / safeRange;
      const y =
        VIEWBOX_HEIGHT -
        normalizedValue * (VIEWBOX_HEIGHT - STROKE_WIDTH * 2) -
        STROKE_WIDTH;

      return `${x},${y}`;
    })
    .join(' ');
}

const MiniTrendSparkline = ({ data, color }) => {
  const points = useMemo(() => buildSparklinePoints(data), [data]);

  if (!points) {
    return null;
  }

  return (
    <svg
      viewBox={`0 0 ${VIEWBOX_WIDTH} ${VIEWBOX_HEIGHT}`}
      className='h-10 w-24'
      aria-hidden='true'
      preserveAspectRatio='none'
    >
      <polyline
        points={points}
        fill='none'
        stroke={color}
        strokeWidth={STROKE_WIDTH}
        strokeLinecap='round'
        strokeLinejoin='round'
      />
    </svg>
  );
};

export default MiniTrendSparkline;
