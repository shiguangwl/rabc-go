<script setup>
import {ColumnHeightOutlined, PlusOutlined, ReloadOutlined, SettingOutlined} from '@ant-design/icons-vue'
import {
  createRoleApi,
  deleteRoleApi,
  getAdminApiApi,
  getRolePermissionsApi,
  getRolesApi,
  updateRoleApi,
  updateRolePermissionsApi
} from '~@/api/common/admin'
import {getAdminMenusApi} from "~/api/common/menu.js";
import {useUserStore} from "~/stores/user.js";


const message = useMessage()
const columns = shallowRef([
  {
    title: '#',
    dataIndex: 'id',
  },
  {
    title: '角色唯一标识',
    dataIndex: 'sid',
  },
  {
    title: '角色名称',
    dataIndex: 'name',
  },
  {
    title: '创建时间',
    dataIndex: 'createdAt',
  },
  {
    title: '更新时间',
    dataIndex: 'updatedAt',
  },
  {
    title: '操作',
    dataIndex: 'action',
  },
])
const loading = shallowRef(false)
const pagination = reactive({
  pageSize: 10,
  pageSizeOptions: ['10', '20', '30', '40'],
  current: 1,
  total: 100,
  showSizeChanger: true,
  showQuickJumper: true,
  showTotal: total => `总数据位：${total}`,
  onChange(current, pageSize) {
    pagination.pageSize = pageSize
    pagination.current = current
    init()
  },
})
const dataSource = shallowRef([])
const rolePermissions = shallowRef([])
const adminApis = shallowRef([])
const formModelSearch = reactive({
  name: "",
  sid: "",
})
const resetFormSearch = () => {
  Object.assign(formModelSearch, {
    name: "",
    sid: "",
  });
};
const formModel = reactive({
  id: 0,
  name: "",
  sid: "",
  createdAt: "",
  updatedAt: "",
})
const formModelPermission = reactive({
  id: 0,
  name: "",
  sid: "",
  createdAt: "",
  updatedAt: "",
})
const resetForm = () => {
  Object.assign(formModel, {
    id: 0,
    name: "",
    sid: "",
    createdAt: "",
    updatedAt: "",
  });
};
const rules = {
  name: [
    {
      required: true,
      message: 'Please enter name',
    },
  ],
  sid: [
    {
      required: true,
      message: 'please enter sid',
    },
  ],
};
const tableSize = ref(['large'])
const sizeItems = ref([
  {
    key: 'large',
    label: '默认',
    title: '默认',
  },
  {
    key: 'middle',
    label: '中等',
    title: '中等',
  },
  {
    key: 'small',
    label: '紧凑',
    title: '紧凑',
  },
])
const activeKey = ref('1')
const checkedKeysApi = ref([]);
const checkedKeysMenu = ref([]);
const menuData = shallowRef([])
const open = ref(false)
const openPermission = ref(false)
const permissionRole = ref({})
const options = computed(() => {
  return columns.value.map((item) => {
    if (item.dataIndex === 'action') {
      return {
        label: item.title,
        value: item.dataIndex,
        disabled: true,
      }
    }
    return {
      label: item.title,
      value: item.dataIndex,
    }
  })
})
const dropdownVisible = ref(false)
const getCheckList = computed(() => columns.value.map(item => item.dataIndex))
const state = reactive({
  indeterminate: false,
  checkAll: true,
  checkList: getCheckList.value,
})
const onClose = () => {
  open.value = false
  openPermission.value = false
}

async function init() {
  if (loading.value)
    return
  loading.value = true
  try {
    const {data} = await getRolesApi({
      ...formModelSearch,
      page: pagination.current,
      pageSize: pagination.pageSize,
    })
    dataSource.value = data.list ?? []
    pagination.total = data.total ?? 0

  } catch (e) {
    console.log(e)
  } finally {
    loading.value = false
  }
}

async function onSearch() {
  pagination.current = 1
  await init()
}

async function onReset() {
  formModel.name = ""
  formModel.sid = ""
  await init()
}

function handleClose() {
  open.value = false
  openPermission.value = false
  permissionRole.value = {}
  onSearch()
}

async function handleCreate(record) {
  resetForm()
  open.value = true
}

async function handleUpdate(record) {
  resetForm()
  Object.assign(formModel, record)
  open.value = true
}

async function handlePermission(record) {
  permissionRole.value = record
  resetForm()
  const {data} = await getAdminMenusApi({})
  menuData.value = formatToTree(data.list) ?? []

  const {data: rolePermissionsData} = await getRolePermissionsApi({
    role: record.sid,
  })
  rolePermissions.value = rolePermissionsData.list ?? []
  checkedKeysApi.value = rolePermissions.value.filter(item => item.startsWith("api:"));
  checkedKeysMenu.value = rolePermissions.value.filter(item => item.startsWith("menu:"));


  const {data: adminApisData} = await getAdminApiApi({
    page: 1,
    pageSize: 10000,
  })
  adminApis.value = apiDataFormatToTree(adminApisData.list) ?? []


  openPermission.value = true
}

const apiDataFormatToTree = (data) => {
// 使用 Map 来分组数据
  const groupMap = new Map();

  // 遍历原始数据，按 group 分组
  data.forEach(item => {
    const groupName = item.group;
    if (!groupMap.has(groupName)) {
      groupMap.set(groupName, []);
    }
    // 将去掉 group 字段后的数据放入对应分组
    // const { group, ...rest } = item;
    item.key = "api:" + item.path + "," + item.method
    item.title = item.name
    groupMap.get(groupName).push(item);
  });

  // 转换为树形结构
  const treeData = [];
  groupMap.forEach((children, groupName) => {
    treeData.push({
      key: groupName,
      title: groupName,
      group: groupName,
      children: children
    });
  });

  return treeData;
}
const formatToTree = (arr) => {
  // 创建节点映射
  const map = new Map();
  arr.forEach(item => map.set(item.id, {...item}));

  // 创建结果数组
  const result = [];

  // 遍历所有节点
  arr.forEach(item => {
    const node = map.get(item.id);
    node.key = "menu:" + node.path + ",read"

    // 如果是顶级节点（parentId 为 0）或父节点不存在
    if (item.parentId === 0 || !map.has(item.parentId)) {
      result.push(node);
    } else {
      // 找到父节点并添加子节点
      const parent = map.get(item.parentId);
      if (parent) {
        // 如果父节点还没有 children，则初始化
        if (!parent.children) {
          parent.children = [];
        }
        parent.children.push(node);
      }
    }
  });
  return result;
}

async function handleDelete(record) {
  const close = message.loading('删除中......')
  try {
    const res = await deleteRoleApi({
      id: record.id,
    })
    if (res.code === 0)
      await init()
    message.success('删除成功')
  } catch (e) {
    console.log(e)
  } finally {
    close()
  }
}


function handleSizeChange(e) {
  tableSize.value[0] = e.key
}

function filterAction(value) {
  return columns.value.filter((item) => {
    if (value.includes(item.dataIndex))
      return true

    return false
  })
}

const filterColumns = ref(filterAction(getCheckList.value))

function handleCheckAllChange(e) {
  Object.assign(state, {
    checkList: e.target.checked ? getCheckList.value : [],
    indeterminate: true,
  })
  filterColumns.value = e.target.checked ? filterAction(getCheckList.value) : filterColumns.value.filter(item => item.dataIndex === 'action')
}

watch(
    () => state.checkList,
    (val) => {
      state.indeterminate = !!val.length && val.length < getCheckList.value.length
      state.checkAll = val.length === getCheckList.value.length
    },
)

function handleResetChange() {
  state.checkList = getCheckList.value
  filterColumns.value = filterAction(getCheckList.value)
}

function handleCheckChange(value) {
  const filterValue = filterAction(value)
  filterColumns.value = filterValue
}

onMounted(() => {
  init()
})

async function onSubmit() {
  const close = message.loading('提交中......')
  try {
    let res = {}
    if (formModel.id > 0) {
      res = await updateRoleApi({
        ...formModel,
      })
    } else {
      res = await createRoleApi({
        ...formModel,
      })
    }

    if (res.code === 0) {
      await init()
      open.value = false
      if (formModel.id > 0) {
        message.success('更新成功')
      } else {
        message.success('创建成功')
      }
    }

  } catch (e) {
    console.log(e)
  } finally {
    close()
  }


}

async function onSubmitPermission() {
  const close = message.loading('提交中......')
  try {

    let res = await updateRolePermissionsApi({
      role: permissionRole.value.sid,
      list: [...checkedKeysApi.value, ...checkedKeysMenu.value]
    })
    if (res.code === 0) {
      await init()
      openPermission.value = false
      message.success('更新成功')
      await useUserStore().generateDynamicRoutes()

    }

  } catch (e) {
    console.log(e)
  } finally {
    close()
  }


}


</script>

<template>
  <page-container>
    <a-card mb-4>
      <a-form :model="formModelSearch">
        <a-row :gutter="[15, 0]">
          <a-col :span="8">
            <a-form-item name="desc" label="角色ID">
              <a-input v-model:value="formModelSearch.sid"/>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item name="name" label="角色名称">
              <a-input v-model:value="formModelSearch.name"/>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-space flex justify-end w-full>
              <a-button :loading="loading" type="primary" @click="onSearch">
                查询
              </a-button>
              <a-button :loading="loading" @click="onReset">
                重置
              </a-button>

            </a-space>
          </a-col>

        </a-row>
      </a-form>
    </a-card>
    <a-card title="角色列表">
      <template #extra>
        <a-space size="middle">
          <a-button type="primary" @click="handleCreate">
            <template #icon>
              <PlusOutlined/>
            </template>
            新增
          </a-button>
          <a-tooltip title="刷新">
            <ReloadOutlined @click="onSearch"/>
          </a-tooltip>
          <a-tooltip title="密度">
            <a-dropdown trigger="click">
              <ColumnHeightOutlined/>
              <template #overlay>
                <a-menu v-model:selected-keys="tableSize" :items="sizeItems" @click="handleSizeChange"/>
              </template>
            </a-dropdown>
          </a-tooltip>
          <a-tooltip title="列设置">
            <a-dropdown v-model:open="dropdownVisible" trigger="click">
              <SettingOutlined/>
              <template #overlay>
                <a-card>
                  <template #title>
                    <a-checkbox v-model:checked="state.checkAll" :indeterminate="state.indeterminate"
                                @change="handleCheckAllChange">
                      列选择
                    </a-checkbox>
                  </template>
                  <template #extra>
                    <a-button type="link" @click="handleResetChange">
                      重置
                    </a-button>
                  </template>
                  <a-checkbox-group v-model:value="state.checkList" :options="options"
                                    style="display: flex; flex-direction: column;" @change="handleCheckChange"/>
                </a-card>
              </template>
            </a-dropdown>
          </a-tooltip>
        </a-space>
      </template>

      <a-table :loading="loading" :columns="filterColumns" :data-source="dataSource" :pagination="pagination"
               :size="tableSize[0]">
        <template #bodyCell="scope">
          <template v-if="scope?.column?.dataIndex === 'action'">
            <div flex gap-2>
              <a @click="handlePermission(scope?.record)">
                分配权限
              </a>
              <a @click="handleUpdate(scope?.record)">
                编辑
              </a>
              <a v-if="scope?.record.sid!=='admin'" c-error @click="handleDelete(scope?.record)">
                删除
              </a>
            </div>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-drawer
        :title="formModel.id>0?'编辑':'添加' +'角色'"
        :width="400"
        :open="open"
        :body-style="{ paddingBottom: '80px' }"
        :footer-style="{ textAlign: 'right' }"
        @close="handleClose"
    >
      <a-form :model="formModel" :rules="rules" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="24">
            <a-form-item label="角色标识" name="sid">
              <a-input :disabled="formModel.id>0" v-model:value="formModel.sid" placeholder="唯一标识,创建后不可修改"/>
            </a-form-item>
          </a-col>

        </a-row>
        <a-row :gutter="16">
          <a-col :span="24">
            <a-form-item label="角色名称" name="name">
              <a-input v-model:value="formModel.name" placeholder="角色名称"/>
            </a-form-item>
          </a-col>

        </a-row>

      </a-form>
      <template #extra>
        <a-space>
          <a-button @click="onClose">取消</a-button>
          <a-button type="primary" @click="onSubmit">提交</a-button>
        </a-space>
      </template>
    </a-drawer>
    <a-drawer
        title="分配角色权限"
        :width="600"
        :open="openPermission"
        :body-style="{ paddingBottom: '80px' }"
        :footer-style="{ textAlign: 'right' }"
        @close="handleClose"
    >
      <span>角色：{{ permissionRole.name }}</span>

      <a-tabs v-model:activeKey="activeKey">
        <a-tab-pane key="1" tab="接口权限">
          <a-tree
              defaultExpandAll
              v-model:checkedKeys="checkedKeysApi"
              checkable
              :tree-data="adminApis"
              :fieldNames="{
                title:'name'
              }"
          >
            <template #title="{ group,title, path,method }">
              <span style="display: inline-block;width: 200px">{{ title }}</span>
              <a-tag style="display: inline-block;width: 55px;font-size:11px;text-align: center" v-if="group !== title">
                {{ method }}
              </a-tag>

              <span v-if="group !== title" style="opacity: .65">{{ path }}</span>
            </template>
          </a-tree>

        </a-tab-pane>

        <a-tab-pane key="2" tab="菜单权限">
          <a-tree
              defaultExpandAll
              v-model:checkedKeys="checkedKeysMenu"
              checkable
              :tree-data="menuData"
          >
            <template #title="{ title, key,parentId }" style="position: relative">
              <span style="display: inline-block;min-width: 200px">{{ title }}</span>
            </template>
          </a-tree>

        </a-tab-pane>
      </a-tabs>
      <template #extra>
        <a-space>
          <a-button @click="onClose">取消</a-button>
          <a-button type="primary" @click="onSubmitPermission">提交</a-button>
        </a-space>
      </template>
    </a-drawer>
  </page-container>
</template>
<style lang="less">

</style>