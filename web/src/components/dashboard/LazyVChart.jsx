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

import React, { Suspense, lazy, useEffect, useState } from 'react';

const VChartRuntime = lazy(() =>
  Promise.all([
    import('@visactor/react-vchart'),
    import('./vchartDashboardRuntime'),
  ]).then(([module, vchartRuntimeModule]) => ({
    default: (props) => (
      <module.VChartSimple
        {...props}
        vchartConstrouctor={vchartRuntimeModule.default}
      />
    ),
  })),
);

const chartPlaceholderStyle = {
  width: '100%',
  height: '100%',
};

const LazyVChart = (props) => {
  const [shouldRender, setShouldRender] = useState(false);

  useEffect(() => {
    let frameId;
    let idleId;

    const scheduleRender = () => setShouldRender(true);

    if (typeof window !== 'undefined' && 'requestIdleCallback' in window) {
      idleId = window.requestIdleCallback(scheduleRender);
    } else {
      frameId = window.requestAnimationFrame(scheduleRender);
    }

    return () => {
      if (typeof window !== 'undefined' && 'cancelIdleCallback' in window) {
        window.cancelIdleCallback(idleId);
      }
      if (typeof window !== 'undefined' && frameId) {
        window.cancelAnimationFrame(frameId);
      }
    };
  }, []);

  if (!shouldRender) {
    return <div style={chartPlaceholderStyle} />;
  }

  return (
    <Suspense fallback={<div style={chartPlaceholderStyle} />}>
      <VChartRuntime {...props} />
    </Suspense>
  );
};

export default LazyVChart;
