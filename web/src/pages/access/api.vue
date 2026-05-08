<script setup>
import * as Icons from '@ant-design/icons-vue'
import {ColumnHeightOutlined, PlusOutlined, ReloadOutlined} from '@ant-design/icons-vue'
import {createAdminApiApi, deleteAdminApiApi, getAdminApiApi, updateAdminApiApi} from '~@/api/common/admin'

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
const message = useMessage()
const columns = shallowRef([
  {
    title: '#',
    dataIndex: 'id',

  },
  {
    title: '分组',
    dataIndex: 'group',
  },
  {
    title: 'API名称',
    dataIndex: 'name',
  },
  {
    title: 'API路由',
    dataIndex: 'path',
  },

  {
    title: 'Method',
    dataIndex: 'method',
  },


  {
    title: '更新时间',
    dataIndex: 'updatedAt',
  },
  {
    title: '操作',
    dataIndex: 'action',
    width: 200,
  },
])
const rules = {
  group: [
    {
      required: true,
      message: 'please enter group',
    },
  ],
  method: [
    {
      required: true,
      message: 'please select an method',
    },
  ],
  name: [
    {
      required: true,
      message: 'Please enter name',
    },
  ],
  path: [
    {
      required: true,
      message: 'Please enter path',
    },
  ],
};
const loading = shallowRef(false)
const dataSource = shallowRef([])
const formModelSearch = reactive({
  id: 0,
  path: "",
  name: "",
  group: "",
  method: "",
})

// 定义选中值，初始化为空数组
const selectedValue = ref([]);

// 处理选择变化
const handleChangeGroup = (value) => {
  if (value.length > 1) {
    selectedValue.value = [value[value.length - 1]]; // 只保留最后一个值
  } else {
    selectedValue.value = value; // 更新值
  }
  formModel.group = selectedValue.value[0]
};
const formModel = reactive({
  id: 0,
  path: "",
  name: "",
  group: "",
  method: "GET",
})
const resetForm = () => {
  Object.assign(formModel, {
    id: 0,
    path: "",
    name: "",
    group: "",
    method: "GET",
  });
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
const open = ref(false)
const groupOptions = ref([])
const defaultExpandedRowKeys = ref([])
const formatToTree = (data) => {
// 使用 Map 来分组数据
  const groupMap = new Map();

  // 遍历原始数据，按 group 分组
  data.forEach(item => {
    const groupName = item.group;
    if (!groupMap.has(groupName)) {
      groupMap.set(groupName, []);
      if (!defaultExpandedRowKeys.value.includes(groupName)) {
        defaultExpandedRowKeys.value.push(groupName);
      }
    }
    // 将去掉 group 字段后的数据放入对应分组
    // const { group, ...rest } = item;
    groupMap.get(groupName).push(item);
  });

  // 转换为树形结构
  const treeData = [];
  groupMap.forEach((children, groupName) => {
    treeData.push({
      key: groupName,
      group: groupName,
      children: children
    });
  });

  return treeData;
}

async function init() {
  if (loading.value)
    return
  loading.value = true
  try {
    const {data} = await getAdminApiApi({
      ...formModelSearch,
      page: pagination.current,
      pageSize: pagination.pageSize,
    })
    dataSource.value = formatToTree(data.list) ?? []
    pagination.total = data.total ?? 0
    groupOptions.value = data.groups ?? []
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
    id: 0,
    path: "",
    name: "",
    group: "",
    method: "",
  });

  await init()
}

const onClose = () => {
  open.value = false
}

async function onSubmit() {
  const close = message.loading('提交中......')
  try {
    let res = {}
    if (formModel.id > 0) {
      res = await updateAdminApiApi({
        ...formModel,
      })
    } else {
      res = await createAdminApiApi({
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

async function handleCreate(record) {
  resetForm()
  selectedValue.value = []
  if (record.group) {
    formModel.group = record.group
    selectedValue.value = [record.group]
  }
  open.value = true

}

async function handleUpdate(record) {
  resetForm()
  selectedValue.value = [record.group]
  Object.assign(formModel, record)
  open.value = true
}

async function handleDelete(record) {
  // 如果存在子节点，则不允许删除
  if (record.children && record.children.length > 0) {
    message.error('存在子节点，不允许删除')
    return
  }
  const close = message.loading('删除中......')
  try {
    const res = await deleteAdminApiApi({
      id: record.id
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

function handleClose() {
  open.value = false
  onSearch()
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


onMounted(() => {
  init()
})
</script>

<template>
  <page-container>
    <a-card mb-4>
      <a-form :model="formModelSearch">
        <a-row :gutter="[15, 0]">
          <a-col :span="4">
            <a-form-item name="group" label="API分组">
              <a-input style="display: none" v-model:value="formModelSearch.group" placeholder="描述API的功能"/>

              <a-select
                  v-model:value="selectedValue"
                  mode="tags"
                  :max-tag-count="1"
                  @change="handleChangeGroup"
              >
                <a-select-option v-for="item in groupOptions" :key="item" :value="item">
                  {{ item }}
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="4">
            <a-form-item name="name" label="API名称">
              <a-input v-model:value="formModelSearch.name"/>
            </a-form-item>
          </a-col>
          <a-col :span="4">
            <a-form-item name="name" label="API路由">
              <a-input v-model:value="formModelSearch.path"/>
            </a-form-item>
          </a-col>
          <a-col :span="4">
            <a-form-item name="name" label="Method">
              <a-select v-model:value="formModelSearch.method" placeholder="">
                <a-select-option value="">All</a-select-option>
                <a-select-option value="GET">GET</a-select-option>
                <a-select-option value="POST">POST</a-select-option>
                <a-select-option value="PUT">PUT</a-select-option>
                <a-select-option value="DELETE">DELETE</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="4">
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
    <a-card title="接口列表">
      <template #extra>
        <a-space size="middle">
          <a-button type="primary" @click="handleCreate">
            <template #icon>
              <PlusOutlined/>
            </template>
            添加API
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
        </a-space>
      </template>
      <a-table :loading="loading" :columns="columns" :data-source="dataSource"
               :defaultExpandedRowKeys="defaultExpandedRowKeys"
               :size="tableSize[0]" :pagination="pagination" :expand-column-width="100">
        <template #bodyCell="column">
          <template v-if="column?.column?.dataIndex === 'name'">
            <div flex gap-2>
              <span>
                {{ column?.record.name }}
              </span>
            </div>
          </template>
          <template v-if="column?.column?.dataIndex === 'method'">
            <div flex gap-2>
              <a-tag v-if="column?.record.method==='GET'" color="success">
                {{ column?.record.method }}
              </a-tag>
              <a-tag v-if="column?.record.method==='POST'" color="warning">
                {{ column?.record.method }}
              </a-tag>
              <a-tag v-if="column?.record.method==='PUT'" color="pink">
                {{ column?.record.method }}
              </a-tag>
              <a-tag v-if="column?.record.method==='DELETE'" color="error">
                {{ column?.record.method }}
              </a-tag>
            </div>
          </template>
          <template v-if="column?.column?.dataIndex === 'action'">
            <div flex gap-2>
              <a v-if="column?.record.key" @click="handleCreate(column?.record)">
                添加API
              </a>
              <a v-if="!column?.record.key" @click="handleUpdate(column?.record)">
                编辑
              </a>
              <a v-if="!column?.record.key" c-error @click="handleDelete(column?.record)">
                删除
              </a>
            </div>
          </template>
          <template v-if="column?.column?.dataIndex === 'title'">
            <div gap-2>
              <component :is="Icons[column.record.icon]" v-if="column.record.icon"/>
              {{ column.record.title }}
            </div>
          </template>
        </template>
      </a-table>
    </a-card>


    <a-drawer
        :title="(formModel.id>0?'编辑':'添加') +'API'"
        :width="550"
        :open="open"
        :body-style="{ paddingBottom: '80px' }"
        :footer-style="{ textAlign: 'right' }"
        @close="handleClose"
    >
      <a-form :model="formModel" :rules="rules" layout="horizontal">
        <a-row :gutter="16">
          <a-col :span="24">
            <a-form-item label="API分组" name="group">
              <a-input style="display: none" v-model:value="formModel.group" placeholder="描述API的功能"/>

              <a-select
                  v-model:value="selectedValue"
                  mode="tags"
                  placeholder="分组不存在则自动创建"
                  :max-tag-count="1"
                  @change="handleChangeGroup"
                  style="width: 200px"
              >
                <a-select-option v-for="item in groupOptions" :key="item" :value="item">
                  {{ item }}
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="24">
            <a-form-item label="API名称" name="name">
              <a-input v-model:value="formModel.name" placeholder="描述API的功能"/>
            </a-form-item>


          </a-col>

          <a-col :span="24">
            <a-form-item label="Method" name="method">
              <a-select v-model:value="formModel.method" placeholder="">
                <a-select-option value="GET">GET</a-select-option>
                <a-select-option value="POST">POST</a-select-option>
                <a-select-option value="PUT">PUT</a-select-option>
                <a-select-option value="DELETE">DELETE</a-select-option>
              </a-select>
            </a-form-item>

          </a-col>
          <a-col :span="24">
            <a-form-item label="API地址" name="path">
              <a-input v-model:value="formModel.path" placeholder="示例：/v1/users"/>
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
