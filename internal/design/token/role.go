package token

var (
	GapTight   = Space2
	GapInline  = Space4
	GapRowIcon = Space6
	GapControl = Space8
	GapStack   = Space8
	GapGroup   = Space12
	GapSection = Space16
)

var (
	PadRow          = Space8
	PadView         = Space20
	PadPanel        = Space12
	PadPopover      = Space4
	PadControlX     = Space8
	PadControlEnd   = Space4
	PadButtonX      = Space12
	PadInputX       = Space6
	PadInputY       = Space4
	PadInputEnd     = Space2
	PadBadgeX       = Space4
	PadChipX        = Space6
	PadKeyX         = Space4
	PadNoticeX      = Space8
	PadNoticeY      = Space6
	PadStatusItemX  = Space6
	PadTabX         = Space12
	PadEmpty        = Space24
	PadLoading      = Space12
	IndentStep      = Space8
	OffsetPopover   = Space2
	OffsetQuickOpen = Space4
	OffsetTooltip   = Space6
	PadTooltipX     = Space8
	PadTooltipY     = Space4
)

var (
	RowHeight          = Size{22}
	HeaderHeight       = Size{22}
	ViewTitleHeight    = Size{35}
	ActivityBarHeight  = Size{34}
	TabHeight          = Size{35}
	TabStripInset      = Size{1}
	TitleBarHeight     = Size{36}
	StatusBarHeight    = Size{22}
	ToolbarHeight      = Size{28}
	ControlHeight      = Size{26}
	ControlHeightSmall = Size{22}
	HitTargetMin       = Size{22}
	TooltipHeight      = Size{24}
)

var (
	IconSize      = Size{16}
	IconSizeSmall = Size{14}
	IconSizeGlyph = Size{12}
	IconSizeLarge = Size{32}
)

var (
	IconButtonSize     = Size{22}
	ToggleSize         = Size{22}
	CheckboxSize       = Size{16}
	SwitchWidth        = Size{28}
	SwitchHeight       = Size{16}
	SwitchKnob         = Size{12}
	SplitArrowWidth    = Size{24}
	ActivityItem       = Size{32}
	AvatarSize         = Size{20}
	BadgeHeight        = Size{18}
	BadgeSmall         = Size{14}
	TagHeight          = Size{18}
	KeyCapHeight       = Size{18}
	KeyCapMinWidth     = Size{18}
	DecorationWidth    = Size{16}
	DirtyDotSize       = Size{8}
	PinnedDirtyDotSize = Size{6}
	TwistieWidth       = Size{16}
	SashSize           = Size{4}
	SashCorner         = Size{12}
	ScrollbarSize      = Size{10}
	ProgressHeight     = Size{2}
)

var (
	PaneMinHeight         = Size{66}
	SidebarMinWidth       = Size{170}
	PanelMinHeight        = Size{100}
	TabMinWidth           = Size{80}
	PopoverMaxWidth       = Size{320}
	DialogWidth           = Size{440}
	QuickInputWidth       = Size{600}
	QuickInputMinWidth    = Size{320}
	SearchPopupWidth      = Size{800}
	SearchPopupHeight     = Size{480}
	SearchPopupMinWidth   = Size{480}
	SearchPopupMinHeight  = Size{320}
	SearchPreviewHeight   = Size{220}
	CommandCenterWidth    = Size{600}
	CommandCenterMinWidth = Size{160}
	EmptyStateWidth       = Size{280}
	NoticeMaxWidth        = Size{480}
	TooltipMaxWidth       = Size{300}
	TagMaxWidth           = Size{160}
	PillNameMaxWidth      = Size{100}
)

const AsideMaxPercent float32 = 40

const (
	QuickInputRows  = 20
	SearchPopupRows = 12
)

var (
	RadiusNone    = Radius0
	RadiusInline  = Radius2
	RadiusControl = Radius4
	RadiusCard    = Radius6
	RadiusPopover = Radius6
	RadiusDialog  = Radius6
	RadiusPill    = RadiusFull
)

var (
	BorderWidth          = Stroke1
	SeparatorWidth       = Stroke1
	FocusRingWidth       = Stroke1
	FocusRingOffset      = Stroke1
	FocusRingOffsetOuter = Stroke2
	IndicatorWidth       = Stroke1
	IndentGuideWidth     = Stroke1
	KeyCapBottomWidth    = Stroke2
)

var (
	OpacityHidden     = Opacity{}
	OpacityDisabled   = Opacity{0.4}
	OpacitySeparator  = Opacity{0.4}
	OpacityDragGhost  = Opacity{0.6}
	OpacityTintFill   = Opacity{0.18}
	OpacityTintBorder = Opacity{0.6}
	OpacityFull       = Opacity{1}
)
