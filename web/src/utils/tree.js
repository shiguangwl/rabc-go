export function deepFind(pred) {
  return ([x, ...xs] = []) => x && (pred(x) ? x : deepFind(pred)(x.children) || deepFind(pred)(xs))
}

export function toTree(list = [], {
  idKey = 'id',
  parentKey = 'parentId',
  rootParent = 0,
  decorate,
} = {}) {
  const nodeMap = new Map()
  list.forEach(item => nodeMap.set(item[idKey], { ...item }))

  const roots = []
  list.forEach((item) => {
    const node = nodeMap.get(item[idKey])
    const extra = decorate?.(node, item)
    if (extra && typeof extra === 'object')
      Object.assign(node, extra)

    const parentID = item[parentKey]
    if (parentID === rootParent || !nodeMap.has(parentID)) {
      roots.push(node)
      return
    }

    const parent = nodeMap.get(parentID)
    parent.children ??= []
    parent.children.push(node)
  })

  return roots
}

export function groupToTree(list = [], {
  groupKey = 'group',
  groupNode,
  itemNode,
  onGroup,
} = {}) {
  const groupMap = new Map()
  list.forEach((item) => {
    const groupName = item[groupKey] ?? ''
    if (!groupMap.has(groupName)) {
      groupMap.set(groupName, [])
      onGroup?.(groupName)
    }
    groupMap.get(groupName).push(itemNode ? itemNode({ ...item }, item) : { ...item })
  })

  return Array.from(groupMap, ([groupName, children]) => {
    return groupNode
      ? groupNode(groupName, children)
      : {
          key: groupName,
          title: groupName,
          [groupKey]: groupName,
          children,
        }
  })
}
