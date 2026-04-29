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

import React, { Suspense, lazy, useEffect, useRef, useState } from 'react';

let vchartRuntimeImportPromise = null;

function loadVChartRuntime() {
  if (!vchartRuntimeImportPromise) {
    vchartRuntimeImportPromise = Promise.all([
      import('@visactor/react-vchart/esm/VChartSimple.js'),
      import('./vchartDashboardRuntime'),
      import('@visactor/vchart-semi-theme'),
    ]).then(([module, vchartRuntimeModule, themeModule]) => {
      themeModule.initVChartSemiTheme({
        isWatchingThemeSwitch: true,
      });

      return {
        default: (props) => (
          <module.VChartSimple
            {...props}
            vchartConstrouctor={vchartRuntimeModule.default}
          />
        ),
      };
    });
  }

  return vchartRuntimeImportPromise;
}

const VChartRuntime = lazy(loadVChartRuntime);

const chartPlaceholderStyle = {
  width: '100%',
  height: '100%',
};

const LazyVChart = (props) => {
  const placeholderRef = useRef(null);
  const [isVisible, setIsVisible] = useState(false);
  const [shouldRender, setShouldRender] = useState(false);

  useEffect(() => {
    if (shouldRender) {
      return undefined;
    }

    const placeholder = placeholderRef.current;

    if (!placeholder || typeof IntersectionObserver === 'undefined') {
      setIsVisible(true);
      return undefined;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setIsVisible(true);
          observer.disconnect();
        }
      },
      {
        rootMargin: '200px',
      },
    );

    observer.observe(placeholder);

    return () => {
      observer.disconnect();
    };
  }, [shouldRender]);

  useEffect(() => {
    if (!isVisible || shouldRender) {
      return undefined;
    }

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
  }, [isVisible, shouldRender]);

  if (!shouldRender) {
    return <div ref={placeholderRef} style={chartPlaceholderStyle} />;
  }

  return (
    <Suspense fallback={<div style={chartPlaceholderStyle} />}>
      <VChartRuntime {...props} />
    </Suspense>
  );
};

export default LazyVChart;
