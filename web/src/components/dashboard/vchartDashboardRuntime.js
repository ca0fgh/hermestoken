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

import { registerAreaChart } from '@visactor/vchart/esm/chart/area/area';
import { registerBarChart } from '@visactor/vchart/esm/chart/bar/bar';
import { registerCommonChart } from '@visactor/vchart/esm/chart/common/common';
import { registerLineChart } from '@visactor/vchart/esm/chart/line/line';
import { registerPieChart } from '@visactor/vchart/esm/chart/pie/pie';
import { registerCartesianBandAxis } from '@visactor/vchart/esm/component/axis/cartesian/band-axis';
import { registerCartesianLinearAxis } from '@visactor/vchart/esm/component/axis/cartesian/linear-axis';
import { registerCartesianCrossHair } from '@visactor/vchart/esm/component/crosshair/cartesian';
import { registerDiscreteLegend } from '@visactor/vchart/esm/component/legend/discrete/legend';
import { registerTooltip } from '@visactor/vchart/esm/component/tooltip/tooltip';
import { VChart } from '@visactor/vchart/esm/core';

VChart.useRegisters([
  registerLineChart,
  registerAreaChart,
  registerBarChart,
  registerPieChart,
  registerCommonChart,
  registerCartesianLinearAxis,
  registerCartesianBandAxis,
  registerDiscreteLegend,
  registerTooltip,
  registerCartesianCrossHair,
]);

export default VChart;
