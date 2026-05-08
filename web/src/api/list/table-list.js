export async function getListApi(params) {
  return usePost('/list/consult-list', params,{
      customDev: true,
  })
}
export async function deleteApi(id) {
  return useDelete(`/list/${id}`,null,{
      customDev: true,
  })
}
