export function getMenusApi() {
  return useGet('/v1/menus')
}
export function getAdminMenusApi() {
  return useGet('/v1/admin/menus')
}
export function createMenuApi(params) {
  return usePost('/v1/admin/menu',params)
}
export function updateMenuApi(params) {
  return usePut('/v1/admin/menu',params)
}
export function deleteMenusApi(params) {
  return useDelete('/v1/admin/menu',params)
}
