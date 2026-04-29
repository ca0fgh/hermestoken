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

const CRYPTO_OPTION_KEYS = [
  'CryptoPaymentEnabled',
  'CryptoScannerEnabled',
  'CryptoOrderExpireMinutes',
  'CryptoUniqueSuffixMax',
  'CryptoTronEnabled',
  'CryptoTronReceiveAddress',
  'CryptoTronUSDTContract',
  'CryptoTronRPCURL',
  'CryptoTronAPIKey',
  'CryptoTronConfirmations',
  'CryptoBSCEnabled',
  'CryptoBSCReceiveAddress',
  'CryptoBSCUSDTContract',
  'CryptoBSCRPCURL',
  'CryptoBSCConfirmations',
  'CryptoPolygonEnabled',
  'CryptoPolygonReceiveAddress',
  'CryptoPolygonUSDTContract',
  'CryptoPolygonRPCURL',
  'CryptoPolygonConfirmations',
  'CryptoSolanaEnabled',
  'CryptoSolanaReceiveAddress',
  'CryptoSolanaUSDTMint',
  'CryptoSolanaRPCURL',
  'CryptoSolanaConfirmations',
];

const CLEARABLE_CRYPTO_TEXT_KEYS = new Set([
  'CryptoTronReceiveAddress',
  'CryptoTronUSDTContract',
  'CryptoTronRPCURL',
  'CryptoBSCReceiveAddress',
  'CryptoBSCUSDTContract',
  'CryptoBSCRPCURL',
  'CryptoPolygonReceiveAddress',
  'CryptoPolygonUSDTContract',
  'CryptoPolygonRPCURL',
  'CryptoSolanaReceiveAddress',
  'CryptoSolanaUSDTMint',
  'CryptoSolanaRPCURL',
]);

const SECRET_CRYPTO_KEYS = new Set(['CryptoTronAPIKey']);

export const buildCryptoPaymentOptionUpdates = (
  inputs = {},
  formValues = {},
) => {
  const values = {
    ...inputs,
    ...formValues,
  };

  return CRYPTO_OPTION_KEYS.reduce((updates, key) => {
    const rawValue = values[key];
    if (SECRET_CRYPTO_KEYS.has(key) && String(rawValue ?? '').trim() === '') {
      return updates;
    }
    if (rawValue === undefined && !CLEARABLE_CRYPTO_TEXT_KEYS.has(key)) {
      return updates;
    }

    const value = CLEARABLE_CRYPTO_TEXT_KEYS.has(key)
      ? rawValue == null
        ? ''
        : rawValue
      : rawValue;

    updates.push({
      key,
      value: typeof value === 'boolean' ? String(value) : String(value),
    });
    return updates;
  }, []);
};
