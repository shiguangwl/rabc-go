<script setup>
import * as Icons from '@ant-design/icons-vue'
import {ColumnHeightOutlined, PlusOutlined, ReloadOutlined, SettingOutlined} from '@ant-design/icons-vue'
import {createMenuApi, deleteMenusApi, getAdminMenusApi, updateMenuApi} from '~@/api/common/menu'
import {useUserStore} from "~/stores/user.js";

const iconList = Object.keys(Icons).filter((key) => {
  // 检查是否是有效的 Vue 组件（图标通常是函数或对象）
  return !(key === 'default' || key === "getTwoToneColor" || key === "setTwoToneColor" || key === "createFromIconfontCN");
});
const message = useMessage()
const columns = shallowRef([
  {
    title: '#',
    dataIndex: 'id',
    width: 150,

  },
  {
    title: '菜单标题',
    dataIndex: 'title',
    width: 150,
  },
  {
    title: '组件路径',
    dataIndex: 'component',
    width: 200,
  },
  {
    title: '路由标识',
    dataIndex: 'name',
  },
  {
    title: '路由',
    dataIndex: 'path',
    width: 100,
  },

  {
    title: '重定向地址',
    dataIndex: 'redirect',
    width: 200,
  },
  {
    title: 'url',
    dataIndex: 'url',
    width: 200,
  },
  {
    title: '权重',
    dataIndex: 'weight',
    width: 80,
  },
  {
    title: '更新时间',
    dataIndex: 'updatedAt',
    width: 200,
  },
  {
    title: '操作',
    dataIndex: 'action',
    width: 200,
  },
])
const rules = {
  title: [
    {
      required: true,
      message: 'please enter title',
    },
  ],
  component: [
    {
      required: true,
      message: 'please enter component',
    },
  ],
  parentId: [
    {
      required: true,
      message: 'please enter parentId',
    },
  ],
  name: [
    {
      required: true,
      message: 'Please enter name',
    },
  ],
  url: [
    {
      required: true,
      message: 'please enter url',
    },
  ],
  path: [
    {
      required: true,
      message: 'Please select an path',
    },
  ],
};
const loading = shallowRef(false)
const dataSource = shallowRef([])
const formModel = reactive({
  id: 0,
  parentId: 0,
  weight: 0,
  parentTitle: "根菜单",
  path: "",
  name: "",
  title: "",
  component: "",
  locale: "",
  icon: "",
  redirect: "",
  url: "",
  keepAlive: false,
  hideInMenu: false,
})
const resetForm = () => {
  Object.assign(formModel, {
    id: 0,
    parentId: 0,
    weight: 0,
    parentTitle: "根菜单",
    path: "",
    name: "",
    title: "",
    component: "",
    locale: "",
    icon: "",
    redirect: "",
    url: "",
    keepAlive: false,
    hideInMenu: false,
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
const formatToTree = (arr) => {
  // 创建节点映射
  const map = new Map();
  arr.forEach(item => map.set(item.id, {...item}));

  // 创建结果数组
  const result = [];

  // 遍历所有节点
  arr.forEach(item => {
    const node = map.get(item.id);
    node.key=node.id
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
const dropdownVisible = ref(false)
const getCheckList = computed(() => columns.value.map(item => item.dataIndex))
const state = reactive({
  indeterminate: false,
  checkAll: true,
  checkList: getCheckList.value,
})

async function init() {
  if (loading.value)
    return
  loading.value = true
  try {
    const {data} = await getAdminMenusApi({})
    dataSource.value = formatToTree(data.list) ?? []
  } catch (e) {
    console.log(e)
  } finally {
    loading.value = false
  }
}

async function onSearch() {
  await init()
}


async function onSubmit() {
  const close = message.loading('提交中......')
  try {
    let res = {}
    formModel.weight = Number(formModel.weight)
    if (formModel.id > 0) {
      res = await updateMenuApi({
        ...formModel,
      })
    } else {
      res = await createMenuApi({
        ...formModel,
      })
    }

    if (res.code === 0) {
      await init()
      await useUserStore().generateDynamicRoutes()
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
  if (record.id) {
    formModel.parentId = record.id
    formModel.parentTitle = record.title
  }
  open.value = true

}

async function handleUpdate(record) {
  resetForm()
  Object.assign(formModel, record)
  let parent = dataSource.value.find(item => item.id === record.parentId)
  formModel.parentTitle = parent ? parent.title : '根菜单'
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
    const res = await deleteMenusApi({
      id: record.id
    })
    if (res.code === 0){
      await init()
      await useUserStore().generateDynamicRoutes()
      message.success('删除成功')
    }
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
</script>

<template>
  <page-container>
    <a-card title="菜单列表">
      <template #extra>
        <a-space size="middle">
          <a-button type="primary" @click="handleCreate">
            <template #icon>
              <PlusOutlined/>
            </template>
            添加根菜单
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
      <a-table :loading="loading" :columns="filterColumns" :pagination="false" :data-source="dataSource"
               :size="tableSize[0]" :expand-column-width="100">
        <template #bodyCell="column">
          <template v-if="column?.column?.dataIndex === 'action'">
            <div flex gap-2>
              <a v-if="!column?.record.parentId" @click="handleCreate(column?.record)">
                添加子菜单
              </a>
              <a @click="handleUpdate(column?.record)">
                编辑
              </a>
              <a c-error @click="handleDelete(column?.record)">
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
        :title="formModel.id>0?'编辑':'添加' +(formModel.parentId>0?'子菜单':'菜单')"
        :width="720"
        :open="open"
        :body-style="{ paddingBottom: '80px' }"
        :footer-style="{ textAlign: 'right' }"
        @close="handleClose"
    >
      <a-form :model="formModel" :rules="rules" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="组件路径" name="component">
              <a-input v-model:value="formModel.component" placeholder="例：/dashboard/analysis"/>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="路由地址" name="path">
              <a-input v-model:value="formModel.path" placeholder="例：/dashboard/analysis"/>
            </a-form-item>


          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="菜单标题" name="title">
              <a-input v-model:value="formModel.title" placeholder="例：分析页"/>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="路由标识" name="name">
              <a-input v-model:value="formModel.name" placeholder="例：DashboardAnalysis"/>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="父级菜单" name="parentId">
              <a-input disabled v-model:value="formModel.parentTitle"/>
              <a-input style="display:none" v-model:value="formModel.parentId"/>
            </a-form-item>
          </a-col>

          <a-col :span="12">
            <a-form-item label="图标" name="icon">
              <a-select v-model:value="formModel.icon">
                <a-select-option value="">
                  <span style="margin-left: 5px;">无图标</span>
                </a-select-option>
                <a-select-option :value="item" v-for="(item,key) in iconList">
                  <component :is="Icons[item]"/>
                  <span style="margin-left: 5px;">{{ item }}</span>
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="是否保活" name="keepAlive">
              <a-select v-model:value="formModel.keepAlive" placeholder="">
                <a-select-option :value="true">是</a-select-option>
                <a-select-option :value="false">否</a-select-option>
              </a-select>
            </a-form-item>

          </a-col>
          <a-col :span="12">
            <a-form-item label="是否隐藏" name="hideInMenu">
              <a-select v-model:value="formModel.hideInMenu" placeholder="">
                <a-select-option :value="true">是</a-select-option>
                <a-select-option :value="false">否</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="排序权重" name="weight">
              <a-input type="number" v-model:value="formModel.weight" placeholder="权重越大越靠前"/>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="多语言标识" name="locale">
              <a-input v-model:value="formModel.locale" placeholder="置空则使用菜单标题"/>
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
