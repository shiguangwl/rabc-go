export function getPublicConfigsApi() {
  return useGet('/v1/config/public')
}
export function getConfigsApi() {
  return useGet('/v1/admin/configs')
}
export function batchUpdateConfigApi(params) {
  return usePut('/v1/admin/configs', params)
}
export function createConfigApi(params) {
  return usePost('/v1/admin/config', params)
}
export function deleteConfigApi(params) {
  return useDelete('/v1/admin/config', params)
}
