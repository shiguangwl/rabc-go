<script setup>
import {Modal} from 'ant-design-vue'
import {ColumnHeightOutlined, PlusOutlined, ReloadOutlined, SettingOutlined} from '@ant-design/icons-vue'
import {createAdminUserApi, deleteAdminUserApi, getAdminUsersApi, updateAdminUserApi} from '~@/api/common/user'
import {getRolesApi} from "~/api/common/admin.js";


const message = useMessage()
const columns = shallowRef([
  {
    title: '#',
    dataIndex: 'id',
  },
  {
    title: '用户名',
    dataIndex: 'username',
  },
  {
    title: '昵称',
    dataIndex: 'nickname',
  },
  {
    title: '手机号',
    dataIndex: 'phone',
  },
  {
    title: '邮箱',
    dataIndex: 'email',
  },
  {
    title: '角色',
    dataIndex: 'roles',
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
const roleMap = shallowRef({})
const formModelSearch = reactive({
  id: null,
  username: "",
  nickname: "",
  email: "",
  phone: "",
  roles: [],
})
const formModel = reactive({
  id: 0,
  username: "",
  nickname: "",
  password: "",
  changePassword: false,
  email: "",
  phone: "",
  roles: [],
})
const resetForm = () => {
  Object.assign(formModel, {
    id: 0,
    username: "",
    nickname: "",
    password: "",
    changePassword: false,
    email: "",
    phone: "",
    roles: [],
  });
};
const rules = {
  username: [
    {
      required: true,
      message: '请输入用户名',
    },
  ],
  password: [
    {
      required: true,
      message: '请设置密码',
    },
  ],
  roles: [
    {
      required: true,
      message: '请分配角色',
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
const menuData = shallowRef([])
const open = ref(false)
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
}

async function init() {
  if (loading.value)
    return
  loading.value = true
  try {
    const {data: rolesData} = await getRolesApi({
      page: pagination.current,
      pageSize: pagination.pageSize,
    })
    roleMap.value = rolesData.list.reduce((acc, cur) => {
      acc[cur.sid] = cur.name
      return acc
    }, {})
    const {data} = await getAdminUsersApi({
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
  Object.assign(formModelSearch, {
    id: null,
    username: "",
    nickname: "",
    password: "",
    changePassword: false,
    email: "",
    phone: "",
    roles: [],
  });
  await init()
}

function handleClose() {
  open.value = false
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

async function handleDelete(record) {
  const close = message.loading('删除中......')
  try {
    const res = await deleteAdminUserApi({
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
      res = await updateAdminUserApi({
        ...formModel,
      })
    } else {
      res = await createAdminUserApi({
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


</script>

<template>
  <page-container>
    <a-card mb-4>
      <a-form :model="formModelSearch">
        <a-row :gutter="[15, 0]">
          <a-col :span="8">
            <a-form-item name="id" label="用户ID">
              <a-input v-model:value="formModelSearch.id"/>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item name="username" label="用户名称">
              <a-input v-model:value="formModelSearch.username"/>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item name="nickname" label="用户名称">
              <a-input v-model:value="formModelSearch.nickname"/>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item name="email" label="邮箱">
              <a-input v-model:value="formModelSearch.email"/>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item name="phone" label="手机号">
              <a-input v-model:value="formModelSearch.phone"/>
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
    <a-card title="用户列表">
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
          <template v-if="scope?.column?.dataIndex === 'roles'">
            <div flex gap-2>
              <a-tag v-for="item in scope.record.roles" :key="item">{{ roleMap[item] }}</a-tag>
            </div>
          </template>
          <template v-if="scope?.column?.dataIndex === 'action'">
            <div flex gap-2>
              <a @click="handleUpdate(scope?.record)">
                编辑
              </a>
              <a v-if="scope?.record.id>1" c-error @click="handleDelete(scope?.record)">
                删除
              </a>
            </div>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-drawer
        :title="formModel.id>0?'编辑':'添加' +'用户'"
        :width="500"
        :open="open"
        :body-style="{ paddingBottom: '80px' }"
        :footer-style="{ textAlign: 'right' }"
        @close="handleClose"
    >
      <a-form :model="formModel" :rules="rules" layout="horizontal" :label-col="{
  style: {
    width: '85px',
  },
}" >
        <a-row :gutter="16">
          <a-col :span="24">
            <a-form-item label="用户名" name="username">
              <a-input v-model:value="formModel.username" placeholder="用户名"/>
            </a-form-item>
          </a-col>
          <a-col :span="24" v-if="!formModel.id">
            <a-form-item label="密码" name="password">
              <a-input-password v-model:value="formModel.password" placeholder="新密码">
                <template #prefix>
                  <LockOutlined class="site-form-item-icon" />
                </template>
              </a-input-password>
            </a-form-item>
          </a-col>


          <a-col :span="24">
            <a-form-item label="昵称" name="nickname">
              <a-input v-model:value="formModel.nickname" placeholder="昵称"/>
            </a-form-item>
          </a-col>
          <a-col :span="24">
            <a-form-item label="邮箱" name="email">
              <a-input v-model:value="formModel.email" placeholder="邮箱"/>
            </a-form-item>
          </a-col>
          <a-col :span="24">
            <a-form-item label="手机号" name="phone">
              <a-input v-model:value="formModel.phone" placeholder="手机号"/>
            </a-form-item>
          </a-col>



          <a-col :span="24">
            <a-form-item label="分配角色" name="roles">
              <a-select
                  v-model:value="formModel.roles"
                  mode="tags"
                  style="width: 100%"
                  placeholder="选择需要分配的角色"
              >
                <a-select-option :value="index" v-for="(item,index) in roleMap">{{ item }}</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="24" v-if="formModel.id">
            <a-form-item label="设置新密码">
              <a-switch v-model:checked="formModel.changePassword" />
            </a-form-item>
            <a-form-item label="新密码" name="password" v-if="formModel.changePassword">
              <a-input-password v-model:value="formModel.password" placeholder="新密码">
                <template #prefix>
                  <LockOutlined class="site-form-item-icon" />
                </template>
              </a-input-password>
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
  </page-container>
</template>
<style lang="less">

</style>