<template>
  <div>
    <div class="filter-container">
      <el-input v-model="listQuery.name" :placeholder="$t('job.name')" style="width: 200px;" class="filter-item" @keyup.enter.native="handleFilter" />
      <el-button class="filter-item" style="margin-left: 10px;" type="primary" icon="el-icon-search" @click="handleFilter">
        {{ $t('job.search') }}
      </el-button>
      <el-button class="filter-item" style="margin-left: 10px;" type="primary" icon="el-icon-refresh" @click="handleFilter">
        {{ $t('job.refresh') }}
      </el-button>
    </div>

    <el-table
      :key="tableKey"
      v-loading="listLoading"
      :data="list"
      border
      fit
      highlight-current-row
      style="width: 100%; margin-top:10px"
    >
      <el-table-column label="ID" prop="id" align="center" width="80">
        <template slot-scope="scope">
          <span>{{ scope.$index +1 }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.namespace')">
        <template slot-scope="scope">
          <span>{{ scope.row.namespace }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.name')">
        <template slot-scope="scope">
          <span>{{ scope.row.name }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.type')">
        <template slot-scope="scope">
          <span>{{ scope.row.type }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.status')">
        <template slot-scope="scope">
          <span>{{ scope.row.status }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.duration')">
        <template slot-scope="scope">
          <span>{{ scope.row.formatTime }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.cpuHour')">
        <template slot-scope="scope">
          <span>{{ scope.row.coreHour }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.gpuHour')">
        <template slot-scope="scope">
          <span>{{ scope.row.gpuHour }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.replicas')">
        <template slot-scope="scope">
          <span>{{ scope.row.replicas }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.createTime')">
        <template slot-scope="scope">
          <span>{{ scope.row.createTime }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.operator')" align="center" width="150" class-name="small-padding fixed-width">
        <template slot-scope="scope">
          <el-button type="primary" size="mini" @click="handleShowDetail(scope.row.jobId)">
            {{ $t('job.detail') }}
          </el-button>
          <!--
          <el-button v-if="row.status!='deleted'" size="mini" type="danger" @click="handleDelete(row.jobId)">
            删除
          </el-button>
          -->
        </template>
      </el-table-column>
    </el-table>

    <pagination v-show="total>0" :total="total" :page.sync="listQuery.page" :limit.sync="listQuery.limit" @pagination="fetchData" />

  </div>
</template>

<script>
import { fetchServingJobList } from '@/api/job'
import Pagination from '@/components/Pagination'

export default {
  components: { Pagination },
  filters: {
    statusFilter(status) {
      const statusMap = {
        published: 'success',
        draft: 'gray',
        deleted: 'danger'
      }
      return statusMap[status]
    }
  },
  data() {
    return {
      tableKey: 0,
      list: null,
      total: 0,
      listLoading: true,
      listQuery: {
        page: 1,
        limit: 20,
        name: ''
      },
      timer: null
    }
  },
  mounted() {
    this.fetchData()
    this.timer = setInterval(() => {
      setTimeout(this.fetchData, 0)
    }, 60000)
  },
  beforeDestroy() {
    clearInterval(this.timer)
    this.timer = null
  },
  methods: {
    fetchData() {
      this.listLoading = true
      fetchServingJobList(this.listQuery).then(response => {
        this.total = response.data.total
        this.list = response.data.items
        this.listLoading = false
      })
    },
    handleShowDetail(jobId) {
      this.$router.push({
        path: '/job/cost',
        query: {
          jobId: jobId,
          jobType: 'serving'
        }
      })
    },
    handleDelete(jobId) {
      // console.log('handle dleete: ' + jobId)
    },
    handleFilter() {
      this.fetchData()
    }
  }
}
</script>
