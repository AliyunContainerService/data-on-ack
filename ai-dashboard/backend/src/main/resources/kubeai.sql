-- DROP TABLE IF EXISTS `ai_job_instance`;
CREATE TABLE IF NOT EXISTS `ai_job_instance` (
`id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
`job_id` varchar(64) NOT NULL DEFAULT '' COMMENT '任务ID',
`name` varchar(128) NOT NULL COMMENT 'pod名称',
`namespace` varchar(128) CHARACTER SET utf8 COLLATE utf8_general_ci DEFAULT '' COMMENT '命名空间',
`duration` varchar(32) NOT NULL COMMENT '运行时长',
`status` varchar(32) NOT NULL DEFAULT '' COMMENT '状态',
`node_name` varchar(128) NOT NULL DEFAULT '' COMMENT '节点名',
`node_ip` varchar(64) NOT NULL DEFAULT '' COMMENT '节点IP',
`instance_type` varchar(64) NOT NULL DEFAULT '' COMMENT '实例类型',
`resource_type` varchar(32) NOT NULL COMMENT '资源类型：ECS/ECI',
`cpu_core` decimal(10,0) DEFAULT NULL COMMENT 'cpu核数',
`gpu` int(11) DEFAULT '0' COMMENT 'gpu卡数',
`is_spot` tinyint(1) DEFAULT NULL COMMENT '是否spot实例',
`trade_price` decimal(20,6) DEFAULT NULL COMMENT '实际交易价格',
`on_demand_price` decimal(20,6) DEFAULT NULL COMMENT '按量付费价格',
`trade_cost` decimal(20,6) DEFAULT NULL COMMENT '实际产生费用',
`on_demand_cost` decimal(20,6) DEFAULT NULL COMMENT '按量付费费用',
`saved_cost` decimal(20,4) DEFAULT NULL COMMENT '节省比率',
`create_time` datetime DEFAULT NULL COMMENT '创建时间',
`modify_time` datetime DEFAULT NULL COMMENT '更新时间',
PRIMARY KEY (`id`),
KEY `idx_job_id` (`job_id`)
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8 COLLATE=utf8_general_ci;

-- DROP TABLE IF EXISTS `ai_serving_job`;
CREATE TABLE IF NOT EXISTS `ai_serving_job` (
`id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
`job_id` varchar(64) NOT NULL COMMENT '任务ID',
`name` varchar(128) NOT NULL COMMENT '推理服务名称',
`namespace` varchar(128) NOT NULL COMMENT '命名空间',
`duration` varchar(32) NOT NULL COMMENT '运行时间',
`core_hour` decimal(20,4) DEFAULT NULL COMMENT 'cpu核时',
`gpu_hour` decimal(20,4) DEFAULT NULL COMMENT 'gpu卡时',
`status` varchar(32) NOT NULL COMMENT '运行状态',
`type` varchar(32) DEFAULT NULL COMMENT '推理服务类型',
`replicas` int(11) DEFAULT NULL COMMENT '实例数量',
`endpoint` varchar(256) DEFAULT NULL COMMENT '请求地址',
`trade_cost` decimal(20,6) DEFAULT NULL COMMENT '实际费用',
`on_demand_cost` decimal(20,6) DEFAULT NULL COMMENT '按量实例费用',
`saved_cost` decimal(20,4) DEFAULT NULL COMMENT '节省比率',
`create_time` datetime DEFAULT NULL COMMENT '创建时间',
`modify_time` datetime DEFAULT NULL COMMENT '修改时间',
PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8 COLLATE=utf8_general_ci;

-- DROP TABLE IF EXISTS `ai_training_job`;
CREATE TABLE IF NOT EXISTS `ai_training_job` (
`id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
`job_id` varchar(64) NOT NULL COMMENT '作业id',
`name` varchar(128) NOT NULL DEFAULT '' COMMENT '任务名',
`namespace` varchar(128) NOT NULL DEFAULT '' COMMENT '命名空间',
`duration` varchar(32) NOT NULL DEFAULT '' COMMENT '运行时长',
`status` varchar(32) NOT NULL DEFAULT '' COMMENT '任务状态',
`type` varchar(32) NOT NULL DEFAULT '' COMMENT '任务类型',
`core_hour` decimal(20,4) DEFAULT NULL COMMENT 'cpu核时',
`gpu_hour` decimal(20,4) DEFAULT NULL COMMENT 'gpu卡时',
`request_gpus` int(11) NOT NULL COMMENT '请求的gpu数量',
`allocated_gpus` int(11) NOT NULL COMMENT '实际分配的gpu数量',
`trade_cost` decimal(20,6) DEFAULT NULL COMMENT '实际费用',
`on_demand_cost` decimal(20,6) DEFAULT NULL COMMENT '按量付费费用',
`saved_cost` decimal(20,4) DEFAULT NULL COMMENT '节省的成本',
`create_time` datetime DEFAULT NULL COMMENT '创建时间',
`modify_time` datetime NOT NULL COMMENT '修改时间',
PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8 COLLATE=utf8_general_ci;
