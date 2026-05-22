<script setup>
import {
  PlusOutlined,
  QuestionCircleOutlined,
  ReloadOutlined,
} from '@ant-design/icons-vue'
import {
  batchUpdateConfigApi,
  createConfigApi,
  deleteConfigApi,
  getConfigsApi,
} from '~@/api/common/config'

const message = useMessage()
const loading = shallowRef(false)
const saving = shallowRef(false)
// groups 需深层可编辑（每个 item.configValue 双向绑定），故用 ref 而非 shallowRef。
const groups = ref([])
const activeKey = ref('')

const valueTypeOptions = [
  { label: '字符串', value: 'string' },
  { label: '整数', value: 'int' },
  { label: '布尔', value: 'bool' },
  { label: 'JSON', value: 'json' },
]

async function init() {
  if (loading.value)
    return
  loading.value = true
  try {
    const { data } = await getConfigsApi()
    groups.value = data.groups ?? []
    if (
      groups.value.length
      && !groups.value.some(g => g.group === activeKey.value)
    ) {
      activeKey.value = groups.value[0].group
    }
  }
  catch (e) {
    message.error('获取系统配置失败')
  }
  finally {
    loading.value = false
  }
}

async function handleSave(group) {
  if (saving.value)
    return
  saving.value = true
  const close = message.loading('保存中......')
  try {
    const res = await batchUpdateConfigApi({
      list: group.items.map(it => ({
        configKey: it.configKey,
        configValue: it.configValue,
      })),
    })
    if (res.code === 0) {
      message.success('保存成功')
      await init()
    }
  }
  catch (e) {
    message.error('保存配置失败')
  }
  finally {
    saving.value = false
    close()
  }
}

const open = ref(false)
const submitting = shallowRef(false)
const formRef = ref()
const formModel = reactive({
  configKey: '',
  title: '',
  configGroup: '',
  valueType: 'string',
  configValue: '',
  remark: '',
  isPublic: false,
  weight: 0,
})
const rules = {
  configKey: [{ required: true, message: '请输入配置键' }],
  title: [{ required: true, message: '请输入展示名称' }],
  configGroup: [{ required: true, message: '请输入配置分组' }],
  valueType: [{ required: true, message: '请选择值类型' }],
}
function resetForm() {
  Object.assign(formModel, {
    configKey: '',
    title: '',
    configGroup: '',
    valueType: 'string',
    configValue: '',
    remark: '',
    isPublic: false,
    weight: 0,
  })
}
function handleCreate() {
  resetForm()
  open.value = true
}

async function onSubmit() {
  try {
    await formRef.value?.validate()
  }
  catch {
    // 校验失败由 a-form 就地展示错误，无需额外提示。
    return
  }
  if (submitting.value)
    return
  submitting.value = true
  const close = message.loading('提交中......')
  try {
    const res = await createConfigApi({ ...formModel })
    if (res.code === 0) {
      message.success('创建成功')
      open.value = false
      await init()
    }
  }
  catch (e) {
    message.error('创建配置失败')
  }
  finally {
    submitting.value = false
    close()
  }
}

async function handleDelete(item) {
  const close = message.loading('删除中......')
  try {
    const res = await deleteConfigApi({ id: item.id })
    if (res.code === 0) {
      message.success('删除成功')
      await init()
    }
  }
  catch (e) {
    message.error('删除配置失败')
  }
  finally {
    close()
  }
}

onMounted(() => {
  init()
})
</script>

<template>
  <page-container>
    <a-card title="系统配置">
      <template #extra>
        <a-space size="middle">
          <a-button type="primary" @click="handleCreate">
            <template #icon>
              <PlusOutlined />
            </template>
            新增配置
          </a-button>
          <a-tooltip title="刷新">
            <ReloadOutlined @click="init" />
          </a-tooltip>
        </a-space>
      </template>

      <a-spin :spinning="loading">
        <a-empty v-if="!groups.length" description="暂无配置项" />
        <a-tabs v-else v-model:active-key="activeKey">
          <a-tab-pane
            v-for="group in groups"
            :key="group.group"
            :tab="group.group"
          >
            <a-form
              layout="horizontal"
              :label-col="{ style: { width: '160px' } }"
            >
              <a-form-item v-for="item in group.items" :key="item.id">
                <template #label>
                  <a-space :size="4">
                    <span>{{ item.title }}</span>
                    <a-tooltip v-if="item.remark" :title="item.remark">
                      <QuestionCircleOutlined
                        style="color: var(--text-color-2)"
                      />
                    </a-tooltip>
                  </a-space>
                </template>
                <a-row :gutter="[12, 8]" align="middle">
                  <a-col flex="auto">
                    <a-switch
                      v-if="item.valueType === 'bool'"
                      :checked="item.configValue === 'true'"
                      @change="(v) => (item.configValue = v ? 'true' : 'false')"
                    />
                    <a-input-number
                      v-else-if="item.valueType === 'int'"
                      :value="
                        item.configValue === ''
                          ? null
                          : Number(item.configValue)
                      "
                      style="width: 100%"
                      @change="
                        (v) => (item.configValue = v == null ? '' : String(v))
                      "
                    />
                    <a-textarea
                      v-else-if="item.valueType === 'json'"
                      v-model:value="item.configValue"
                      :rows="4"
                    />
                    <a-input v-else v-model:value="item.configValue" />
                  </a-col>
                  <a-col>
                    <a-space :size="8">
                      <a-tag>{{ item.configKey }}</a-tag>
                      <a-tag v-if="item.isPublic" color="blue">
                        公开
                      </a-tag>
                      <a-tag v-if="item.isSystem" color="gold">
                        内置
                      </a-tag>
                      <a-popconfirm
                        v-if="!item.isSystem"
                        title="确定删除该配置项?"
                        @confirm="handleDelete(item)"
                      >
                        <a c-error>删除</a>
                      </a-popconfirm>
                    </a-space>
                  </a-col>
                </a-row>
              </a-form-item>
              <a-form-item :wrapper-col="{ offset: 2 }">
                <a-button
                  type="primary"
                  :loading="saving"
                  @click="handleSave(group)"
                >
                  保存「{{ group.group }}」
                </a-button>
              </a-form-item>
            </a-form>
          </a-tab-pane>
        </a-tabs>
      </a-spin>
    </a-card>

    <a-drawer
      title="新增配置"
      :width="460"
      :open="open"
      :body-style="{ paddingBottom: '80px' }"
      :footer-style="{ textAlign: 'right' }"
      @close="open = false"
    >
      <a-form ref="formRef" :model="formModel" :rules="rules" layout="vertical">
        <a-form-item label="配置键" name="configKey">
          <a-input
            v-model:value="formModel.configKey"
            placeholder="如 site.name，程序读取标识，创建后不建议改"
          />
        </a-form-item>
        <a-form-item label="展示名称" name="title">
          <a-input
            v-model:value="formModel.title"
            placeholder="后台展示用名称"
          />
        </a-form-item>
        <a-form-item label="配置分组" name="configGroup">
          <a-input
            v-model:value="formModel.configGroup"
            placeholder="决定所属 Tab，如 站点设置"
          />
        </a-form-item>
        <a-form-item label="值类型" name="valueType">
          <a-select
            v-model:value="formModel.valueType"
            :options="valueTypeOptions"
          />
        </a-form-item>
        <a-form-item label="配置值" name="configValue">
          <a-textarea v-model:value="formModel.configValue" :rows="3" />
        </a-form-item>
        <a-form-item label="说明" name="remark">
          <a-input v-model:value="formModel.remark" />
        </a-form-item>
        <a-form-item label="权重" name="weight">
          <a-input-number
            v-model:value="formModel.weight"
            style="width: 100%"
          />
        </a-form-item>
        <a-form-item label="允许未登录访问" name="isPublic">
          <a-switch v-model:checked="formModel.isPublic" />
        </a-form-item>
      </a-form>
      <template #extra>
        <a-space>
          <a-button @click="open = false">
            取消
          </a-button>
          <a-button type="primary" :loading="submitting" @click="onSubmit">
            提交
          </a-button>
        </a-space>
      </template>
    </a-drawer>
  </page-container>
</template>
