import { isEmpty } from '@/utils'
import { versionCompare } from '@/utils/validate'
import request from '@/utils/request'

function parseResourceTypes(minMaxObject, resourceTypes) {
  if (minMaxObject === undefined) {
    return
  }
  for (const o of Object.entries(minMaxObject)) {
    var key = o[0]
    if (resourceTypes.root.indexOf(key) < 0) {
      resourceTypes.root.push(key)
    }
  }
}

export function stripUnit(quatity, defaultUnit) {
  if (isEmpty(defaultUnit)) {
    return quatity
  }
  var unitIndex = quatity.indexOf(defaultUnit)
  // that's to say default unit is strictly matched
  if (unitIndex < 0 || unitIndex + defaultUnit.length !== quatity.length) {
    return quatity
  }
  return quatity.substr(0, unitIndex)
}

export function deserializeMinMax(min) {
  if (isEmpty(min) || isEmpty(min.memory)) {
    return min
  }
  min.memory = stripUnit(min.memory, 'M')
  return min
}

export function serializeMinMax(min) {
  if (isEmpty(min) || isEmpty(min.memory)) {
    return min
  }
  var mem = min.memory
  var allNumberExp = /(^(\d{1,})\.?(\d{0,})$)|(^N\/A$)/g
  var match = mem.match(allNumberExp)
  if (match !== null) {
    min.memory = mem + 'M'
  }
  return min
}

export function deserializeNodeName(name, spliter = '.') {
  var nameList = name.split(spliter)
  var prefix = ''
  if (nameList.length > 1) {
    prefix = nameList.slice(0, nameList.length - 1).join(spliter)
  }
  return [prefix, nameList[nameList.length - 1]]
}

export function serializeNodeName(prefix, name) {
  if (!isEmpty(prefix)) {
    return prefix + '.' + name
  }
  return name
}

export function serializeElasticQuotaTree(rootNode, resNode) {
  if (isEmpty(rootNode) || isEmpty(rootNode.name)) {
    return resNode
  }
  var curNode = JSON.parse(JSON.stringify(rootNode))
  curNode.name = serializeNodeName(rootNode.prefix, rootNode.name)
  curNode.min = serializeMinMax(curNode.min)
  curNode.max = serializeMinMax(curNode.max)
  resNode = curNode
  if (isEmpty(rootNode.children)) {
    return resNode
  }
  resNode.children = []
  for (var i = 0; i < rootNode.children.length; i++) {
    const child = rootNode.children[i]
    resNode.children.push(serializeElasticQuotaTree(child, null))
  }
  return resNode
}

export function IsK8SVersionSatisfied(clusterInfo) {
  const k8sVersion = clusterInfo.k8sVersion
  if (versionCompare(k8sVersion, '1.20.0') >= 0) {
    return true
  }
  return false
}

export function parseElasticQuotaTree(elasticQuotaTree, outputResourceTypes, outputNamespaces) {
  if (outputResourceTypes === undefined) {
    outputResourceTypes = { root: [] }
  }
  if (outputNamespaces === undefined) {
    outputNamespaces = { root: [] }
  }

  var curNode = {}
  if (elasticQuotaTree.name === undefined) {
    return [curNode, outputResourceTypes, outputNamespaces]
  }

  var [prefix, name] = deserializeNodeName(elasticQuotaTree.name)
  curNode.name = name
  curNode.prefix = prefix
  curNode.min = elasticQuotaTree.min
  parseResourceTypes(curNode.min, outputResourceTypes)
  curNode.max = elasticQuotaTree.max
  parseResourceTypes(curNode.max, outputResourceTypes)

  if (elasticQuotaTree.namespaces !== undefined) {
    curNode.namespaces = elasticQuotaTree.namespaces
    outputNamespaces.root = [...outputNamespaces.root, ...curNode.namespaces]
  }
  if (!isEmpty(elasticQuotaTree.children)) {
    if (curNode.children === undefined) {
      curNode.children = []
    }
    for (var i = 0; i < elasticQuotaTree.children.length; i++) {
      const child = elasticQuotaTree.children[i]
      var childTree;
      [childTree, outputResourceTypes, outputNamespaces] = parseElasticQuotaTree(child, outputResourceTypes, outputNamespaces)
      curNode.children.push(childTree)
    }
  }
  return [curNode, outputResourceTypes, outputNamespaces]
}

export function doRemovedTableData(root, dName, dPrefix = '') {
  let d = root.children || []
  const p = dPrefix.split('.')
  p.splice(0, 1) // skip root
  let i = 0
  if (p.length > 0) {
    while (i < p.length) {
      d.forEach(e => {
        if (e.name === p[i]) {
          d = e.children || []
          i++
        }
      })
    }
  }
  const fIndex = d.findIndex(e => e.name === dName)
  if (fIndex < 0) {
    console.log('not found node name:', dName, dPrefix)
    return root
  }
  d.splice(fIndex, 1)
  return root
}

export function parseSearchData(type = 'prefix', root, searchName) {
  let result

  const search = (t) => {
    if (t.name === searchName || (t.name || '').includes(searchName)) {
      result = type === 'prefix' ? t.prefix : t
    } else if (t.children) {
      t.children.forEach(d => search(d))
    }
  }

  search(root)

  return result
}

const parseQuotaNodeName = (item, result = []) => {
  if (!item.children || (item.children && item.children.length === 0)) {
    result.push(serializeNodeName(item.prefix, item.name))
  }
  if (item.children) {
    for (const d of item.children) {
      parseQuotaNodeName(d, result)
    }
  }
}

export function extractLeafNamesFromQuotaTree(tree = {}) {
  const quotas = []
  if (tree.spec && tree.spec.root) {
    parseQuotaNodeName(tree.spec.root, quotas)
  }
  return quotas
}

export function fetchElasticQuotaTree(query) {
  return request({
    url: '/group/list',
    method: 'get',
    params: query
  })
}

export function createElasticeQuotaTree(data) {
  return request({
    url: '/group/create',
    method: 'post',
    data: data
  })
}

export function updateElasticeQuotaTree(data, action, oldName, newName, prefix) {
  if (isEmpty(newName)) {
    newName = oldName
  }
  return request({
    url: '/group/update',
    method: 'put',
    params: { oldNodeName: oldName, newNodeName: newName, prefix: prefix, action: action },
    data: data
  })
}
